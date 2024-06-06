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
	method              string
	queryHandlerFn      func(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error)
	adminQueryHandlerFn func(ctx context.Context, db *mongo.Database, paramStrs ...*string) ([]byte, error)
)

var (
	// read methods
	find                   method = "find"
	findOne                method = "findOne"
	aggregate              method = "aggregate"
	distinct               method = "distinct"
	count                  method = "count"
	estimatedDocumentCount method = "estimatedDocumentCount"

	// read methods, admin
	showCollections method = "collections"

	// write methods
	insertOne  method = "insertOne"
	insertMany method = "insertMany"
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

	adminReadActions = map[method]adminQueryHandlerFn{
		showCollections: handleShowCollections,
	}

	writeActions = map[method]queryHandlerFn{
		insertOne:  handleInsertOne,
		insertMany: handleInsertMany,
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

	if strings.HasPrefix(q.QueryLine, "show") {
		matches := regexp.MustCompile(`^show (.*)$`).FindAllStringSubmatch(q.QueryLine, -1)
		if len(matches) < 1 || len(matches[0]) < 2 {
			return nil, fmt.Errorf("Invalid read query type and data: '%+v'", matches)
		}

		handler, ok := adminReadActions[method(matches[0][1])]
		if !ok {
			return nil, fmt.Errorf("Admin read handler not found: %v.", matches[0][1])
		}

		return handler(ctx, db, &matches[0][1])
	} else {
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
		return handler(ctx, coll, &matches[0][3], &cursorOptStr)
	}
}

func handleFind(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
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

	cursorOpts, err := parseCursorOpts(find, *paramStrs[1])
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
		if rowCount == common.GetConfig().MaxRowLimit {
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

func handleFindOne(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
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

func handleAggregate(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}
	f, err := constructBsonArray(params[0])
	if err != nil {
		return nil, err
	}

	// TODO: handle opts
	cursor, err := coll.Aggregate(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", err, f)
	}
	defer cursor.Close(ctx)

	rowCount := 0

	tmpRes := []map[string]any{}
	for cursor.Next(ctx) {
		if rowCount == common.GetConfig().MaxRowLimit {
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

func handleCount(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
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

func handleEstimatedDocCount(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
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

func handleDistinct(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
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
			if strings.HasPrefix(v.(string), "ObjectId") {
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
	}

	return resultMap, nil
}

func constructBsonArray(objArr string) ([]any, error) {
	objArr = strings.TrimPrefix(objArr, "[")
	objArr = strings.TrimSuffix(objArr, "]")

	objs, err := splitCommaSeparatedObjStr(objArr)
	if err != nil {
		return nil, err
	}

	var resArr bson.A
	for _, o := range objs {
		// not calling bson.UnmarshalExtJSON for special field handling
		bsonObj, err := constructBsonObject(o)
		if err != nil {
			return nil, err
		}

		resArr = append(resArr, bsonObj)
	}

	return resArr, nil
}

func QueryTypeMongo(query string) common.QueryType {
	if strings.HasPrefix(query, "show") {
		return common.Read
	}

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

	matches := regexp.MustCompile(`db\.(.*?)\.(.*?)\((.*?)\)`).FindAllStringSubmatch(q.QueryLine, -1)

	if len(matches) < 1 || len(matches[0]) < 3 {
		return nil, fmt.Errorf("Invalid write query type and data: '%+v'", matches)
	}

	coll := db.Collection(matches[0][1])
	handler, ok := writeActions[method(matches[0][2])]
	if !ok {
		return nil, fmt.Errorf("Write handler not found: %v.", matches[0][2])
	}

	return handler(ctx, coll, &matches[0][3])
}

func handleInsertOne(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}

	obj, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}
	insertResult, err := coll.InsertOne(ctx, obj)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(struct {
		InsertedID any `json:"inserted_id"`
	}{insertResult.InsertedID})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), insertResult)
	}
	return res, nil
}

func handleInsertMany(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}

	obj, err := constructBsonArray(params[0])
	if err != nil {
		return nil, err
	}
	insertResult, err := coll.InsertMany(ctx, obj)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(struct {
		InsertedIDs any `json:"inserted_ids"`
	}{insertResult.InsertedIDs})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), insertResult)
	}
	return res, nil
}

func handleUpdateOne(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}
	updateObj, err := constructBsonObject(params[1])
	if err != nil {
		return nil, err
	}
	updateResult, err := coll.UpdateOne(ctx, f, updateObj)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(struct {
		MatchedCount  int64 `json:"matched_count"`
		ModifiedCount int64 `json:"modified_count"`
		UpsertedCount int64 `json:"upserted_count"`
		UpsertedID    any   `json:"upserted_id"`
	}{
		MatchedCount:  updateResult.MatchedCount,
		ModifiedCount: updateResult.ModifiedCount,
		UpsertedCount: updateResult.UpsertedCount,
		UpsertedID:    updateResult.UpsertedID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), updateObj)
	}
	return res, nil
}

func handleUpdateMany(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}
	updateObj, err := constructBsonObject(params[1])
	if err != nil {
		return nil, err
	}
	updateResult, err := coll.UpdateMany(ctx, f, updateObj)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(struct {
		MatchedCount  int64 `json:"matched_count"`
		ModifiedCount int64 `json:"modified_count"`
		UpsertedCount int64 `json:"upserted_count"`
		UpsertedID    any   `json:"upserted_id"`
	}{
		MatchedCount:  updateResult.MatchedCount,
		ModifiedCount: updateResult.ModifiedCount,
		UpsertedCount: updateResult.UpsertedCount,
		UpsertedID:    updateResult.UpsertedID,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), updateObj)
	}
	return res, nil
}

func handleDeleteOne(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}
	deleteResult, err := coll.DeleteOne(ctx, f)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(struct {
		DeletedCount int64 `json:"deleted_count"`
	}{
		DeletedCount: deleteResult.DeletedCount,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), deleteResult)
	}
	return res, nil
}

func handleDeleteMany(ctx context.Context, coll *mongo.Collection, paramStrs ...*string) ([]byte, error) {
	params, err := splitCommaSeparatedObjStr(*paramStrs[0])
	if err != nil {
		return nil, err
	}

	f, err := constructBsonObject(params[0])
	if err != nil {
		return nil, err
	}
	deleteResult, err := coll.DeleteMany(ctx, f)
	if err != nil {
		return nil, err
	}
	res, err := json.Marshal(struct {
		DeletedCount int64 `json:"deleted_count"`
	}{
		DeletedCount: deleteResult.DeletedCount,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), deleteResult)
	}
	return res, nil
}

func splitCommaSeparatedObjStr(input string) (result []string, err error) {
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

func handleShowCollections(ctx context.Context, db *mongo.Database, paramStrs ...*string) ([]byte, error) {
	collections, err := db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), collections)
	}

	return json.Marshal(collections)
}
