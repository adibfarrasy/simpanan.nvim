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

type queryHandlerFn func(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error)

var readActions = map[string]queryHandlerFn{
	"find":                   handleFind,
	"findOne":                handleFindOne,
	"aggregate":              handleAggregate,
	"distinct":               handleDistinct,
	"count":                  handleCount,
	"estimatedDocumentCount": handleEstimatedDocCount,
}

func ExecuteMongoQuery(q common.QueryMetadata) ([]byte, error) {
	// TODO: handle passing previousResults
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

	matches := regexp.MustCompile(`db\.(.*?)\.(.*?)\((.*)\)`).FindAllStringSubmatch(q.QueryLine, -1)
	if len(matches) < 1 || len(matches[0]) < 4 {
		return nil, fmt.Errorf("Invalid read query type and data: '%+v'", matches)
	}

	coll := db.Collection(matches[0][1])
	handler, ok := readActions[matches[0][2]]
	if !ok {
		return nil, fmt.Errorf("Read handler not found: %v.", matches[0][2])
	}
	return handler(ctx, coll, matches[0][3])
}

func handleFind(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error) {
	f, err := constructJSONFilter(filter)
	if err != nil {
		return nil, err
	}

	cursor, err := coll.Find(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", err, f)
	}
	defer cursor.Close(ctx)

	tmpRes := []map[string]any{}
	for cursor.Next(ctx) {
		var result map[string]any
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		tmpRes = append(tmpRes, result)
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

func handleFindOne(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error) {
	f, err := constructJSONFilter(filter)
	if err != nil {
		return nil, err
	}
	var result bson.M
	err = coll.FindOne(ctx, f).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", err, f)
	}
	res, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("%s: %v.", err.Error(), result)
	}
	return res, nil
}

func handleAggregate(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error) {
	// TODO: implement this
	return nil, nil
}

func handleCount(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error) {
	f, err := constructJSONFilter(filter)
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

func handleEstimatedDocCount(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error) {
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

func handleDistinct(ctx context.Context, coll *mongo.Collection, filter string) ([]byte, error) {
	fieldAndFilter := strings.Split(filter, ",")
	fieldName := strings.ReplaceAll(fieldAndFilter[0], "\"", "")
	var f any
	if len(fieldAndFilter) > 1 {
		f2, err := constructJSONFilter(fieldAndFilter[1])
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

func constructJSONFilter(filterStr string) (any, error) {
	if filterStr == "{}" {
		return bson.D{}, nil
	}

	var filterMap bson.M
	err := bson.UnmarshalExtJSON([]byte(filterStr), false, &filterMap)
	if err != nil {
		return bson.D{}, fmt.Errorf("%s: %s.", err.Error(), filterStr)
	}

	v, ok := filterMap["_id"]
	if ok {
		replacer := strings.NewReplacer("'", "", "(", "", ")", "")
		hex := replacer.Replace(strings.Split(fmt.Sprintf("%v", v), "ObjectId")[1])
		id, err := primitive.ObjectIDFromHex(hex)
		if err != nil {
			return bson.D{}, fmt.Errorf("%s: %s.", err.Error(), hex)
		}
		filterMap["_id"] = id
	}

	return filterMap, nil
}

func QueryTypeMongo(query string) common.QueryType {
	matches := regexp.MustCompile(`db\..*?\.(.*?)\(.*\)`).FindStringSubmatch(query)
	if len(matches) < 2 {
		return common.QueryType("")
	}
	if _, ok := readActions[matches[1]]; ok {
		return common.Read
	}

	return common.Write
}
