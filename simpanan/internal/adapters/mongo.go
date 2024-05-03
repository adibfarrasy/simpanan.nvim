package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"simpanan/internal/common"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	method         string
	queryHandlerFn func(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error)
)

var (
	// read methods
	find                   method = "find"
	findOne                method = "findOne"
	aggregate              method = "aggregate"
	distinct               method = "distinct"
	count                  method = "count"
	estimatedDocumentCount method = "estimatedDocumentCount"

	// write methods
	insert     method = "insert"
	insertOne  method = "insertOne"
	insertMany method = "insertMany"
	update     method = "update"
	updateOne  method = "updateOne"
	updateMany method = "updateMany"
	deleteOne  method = "deleteOne"
	deleteMany method = "deleteMany"

	readActions = map[method]queryHandlerFn{
		find:                   handleFind,
		findOne:                handleFindOne,
		aggregate:              handleAggregate,
		distinct:               handleDistinct,
		count:                  handleCount,
		estimatedDocumentCount: handleEstimatedDocCount,
	}

	writeActions = map[method]queryHandlerFn{
		insert:     handleInsert,
		insertOne:  handleInsertOne,
		insertMany: handleInsertMany,
		update:     handleUpdate,
		updateOne:  handleUpdateOne,
		updateMany: handleUpdateMany,
		deleteOne:  handleDeleteOne,
		deleteMany: handleDeleteMany,
	}
)

func ExecuteMongoReadQuery(q common.QueryMetadata) ([]byte, error) {
	opt := options.Client().ApplyURI(q.Conn)

	ctx := context.Background()
	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	db := client.Database(strings.Split(q.Conn, "/")[3])

	matches := regexp.MustCompile(`db\.(.*?)\.(.*?)\((.*?)\)(\..*)?$`).FindAllStringSubmatch(q.QueryLine, -1)

	if len(matches) < 1 || len(matches[0]) < 4 {
		return nil, fmt.Errorf("Invalid read query type and data: '%+v'", matches)
	}

	coll := db.Collection(matches[0][1])
	handler, ok := readActions[method(matches[0][2])]
	if !ok {
		return nil, fmt.Errorf("Read handler not found: %v.", matches[0][2])
	}

	cursorOptStr := ""
	if len(matches[0]) == 5 {
		cursorOptStr = matches[0][4]
	}
	return handler(ctx, coll, matches[0][3], cursorOptStr)
}

func handleFind(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	params, err := splitMethodParamStr(methodParamStr)
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}

	opts := options.Find()
	if len(params) > 1 {
		o, err := constructBsonObject(params[1])
		if err != nil {
			return nil, err
		}

		opts.SetProjection(o)
	}

	cursorOpts, err := parseCursorOpts(find, cursorOptStr)
	if err != nil {
		return nil, err
	}
	for _, co := range cursorOpts {
		newOpt, err := co.Apply(opts)
		if err != nil {
			return nil, err
		}
		tmp, ok := newOpt.(*options.FindOptions)
		if !ok {
			return nil, fmt.Errorf("Failed parsing FindOptions for %v", co)
		}
		opts = tmp
	}

	cursor, err := coll.Find(ctx, f, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", err, f)
	}
	defer cursor.Close(ctx)

	rowCount := 0

	tmpRes := []map[string]any{}
	for cursor.Next(ctx) {
		if rowCount == common.MaxDocumentLimit {
			break
		}

		var result map[string]any
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		tmpRes = append(tmpRes, result)
		rowCount++
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	res, err := json.Marshal(tmpRes)
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), tmpRes)
	}
	return res, nil
}

func handleFindOne(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	params, err := splitMethodParamStr(methodParamStr)
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}

	opts := options.FindOne()
	if len(params) > 1 {
		o, err := constructBsonObject(params[1])
		if err != nil {
			return nil, err
		}

		opts.SetProjection(o)
	}

	var result bson.M
	err = coll.FindOne(ctx, f, opts).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", err, f)
	}
	res, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), result)
	}
	return res, nil
}

func handleAggregate(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}

func handleCount(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	params, err := splitMethodParamStr(methodParamStr)
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}
	count, err := coll.CountDocuments(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", err, f)
	}
	res, err := json.Marshal(count)
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), count)
	}
	return res, nil
}

func handleEstimatedDocCount(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	count, err := coll.EstimatedDocumentCount(ctx)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(count)
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), count)
	}
	return res, nil
}

func handleDistinct(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	params, err := splitMethodParamStr(methodParamStr)
	if err != nil {
		return nil, err
	}

	fieldAndFilter := strings.Split(params[0], ",")
	fieldName := strings.ReplaceAll(fieldAndFilter[0], "\"", "")
	var f any
	if len(fieldAndFilter) > 1 {
		f2, err := constructBsonObject(fieldAndFilter[1])
		if err != nil {
			return nil, err
		}
		f = f2
	} else {
		f = bson.D{}
	}
	values, err := coll.Distinct(ctx, fieldName, f)
	if err != nil {
		return nil, fmt.Errorf("%s: %s - %v", err.Error(), fieldName, f)
	}
	res, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), values)
	}
	return res, nil
}

func constructBsonObject(objStr string) (any, error) {
	if objStr == "{}" {
		return bson.D{}, nil
	}

	var resultMap bson.M
	err := bson.UnmarshalExtJSON([]byte(objStr), false, &resultMap)
	if err != nil {
		if strings.Contains(err.Error(), "invalid JSON input") {
			return bson.D{}, fmt.Errorf("%s: %s.\nHint: are all the key-value string fields enclosed by quotes?", err.Error(), objStr)
		}
		return bson.D{}, fmt.Errorf("%s: %s.", err.Error(), objStr)
	}

	v, ok := resultMap["_id"]
	if ok {
		switch v.(type) {
		case string:
			// special handling for ObjectId type
			replacer := strings.NewReplacer("'", "", "(", "", ")", "")
			hex := replacer.Replace(strings.Split(fmt.Sprintf("%v", v), "ObjectId")[1])
			id, err := primitive.ObjectIDFromHex(hex)
			if err != nil {
				return bson.D{}, fmt.Errorf("%s: %s.", err.Error(), hex)
			}
			resultMap["_id"] = id
		}
	}

	return resultMap, nil
}

func QueryTypeMongo(query string) common.QueryType {
	matches := regexp.MustCompile(`db\..*?\.(.*?)\(.*\)`).FindStringSubmatch(query)
	if len(matches) < 2 {
		return common.QueryType("")
	}
	if _, ok := readActions[method(matches[1])]; ok {
		return common.Read
	}

	if _, ok := writeActions[method(matches[1])]; ok {
		return common.Write
	}

	return common.QueryType("")
}

func ExecuteMongoWriteQuery(q common.QueryMetadata) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}

func handleInsert(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleInsertOne(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleInsertMany(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleUpdate(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleUpdateOne(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleUpdateMany(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleDeleteOne(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}
func handleDeleteMany(ctx context.Context, coll *mongo.Collection, methodParamStr string, cursorOptStr string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}

func splitMethodParamStr(input string) (result []string, err error) {
	if len(input) == 0 {
		err = fmt.Errorf("splitMethodParamStr: missing input string")
		return
	}

	pairMap := map[rune]rune{
		'}': '{',
		')': '(',
		']': '[',
	}
	stack := []rune{}
	acc := []rune{}
	for i, c := range input {
		if c != ' ' {
			acc = append(acc, c)
		}
		switch c {
		case '{', '(', '[':
			stack = append(stack, c)
		case '}', ')', ']':
			if len(stack) == 0 {
				err = fmt.Errorf("invalid character at index %d", i)
				return
			} else {
				if stack[len(stack)-1] != pairMap[c] {
					err = fmt.Errorf("invalid parentheses at index %d", i)
					return
				}
				stack = stack[:len(stack)-1]
			}
		case ',':
			if len(stack) == 0 {
				result = append(result, string(acc))
				acc = []rune{}
			}
		}
	}
	if len(acc) > 0 {
		result = append(result, string(acc))
	}
	return
}
