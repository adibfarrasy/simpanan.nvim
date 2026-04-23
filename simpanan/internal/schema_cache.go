package internal

import (
	"context"
	"database/sql"
	"fmt"
	"simpanan/internal/adapters"
	"simpanan/internal/common"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SchemaCache holds the introspected shape of one connection's backing
// database at a point in time. Per spec, no SchemaCache exists for
// redis or jq connections (invariant NoSchemaCacheForRedisOrJq).
type SchemaCache struct {
	ConnectionLabel    string           `json:"connection_label"`
	PopulatedAt        *time.Time       `json:"populated_at,omitempty"`
	LastRefreshAttempt *time.Time       `json:"last_refresh_attempt,omitempty"`
	Databases          []DatabaseSchema `json:"databases"`
}

// DatabaseSchema represents one database (SQL) or one logical Mongo
// database. Tables and Collections are mutually exclusive in practice
// but modelled as two disjoint fields so a single type covers both
// adapter families.
type DatabaseSchema struct {
	Name        string             `json:"name"`
	Tables      []TableSchema      `json:"tables,omitempty"`
	Collections []CollectionSchema `json:"collections,omitempty"`
}

type TableSchema struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type CollectionSchema struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// mongoSampleSize is the number of documents sampled per Mongo
// collection when inferring top-level field names.
const mongoSampleSize int64 = 100

// mongoIntrospectTimeout caps the total time IntrospectMongo will spend
// connecting and listing collections. Per-collection Find calls share
// this budget.
const mongoIntrospectTimeout = 30 * time.Second

// systemMongoDatabases are skipped during introspection because they
// hold internal Mongo state, not user data.
var systemMongoDatabases = map[string]struct{}{
	"admin":  {},
	"local":  {},
	"config": {},
}

// IntrospectSchema connects to the given URI and returns a fresh
// SchemaCache. Redis and jq are ineligible by spec and return
// (nil, nil) without an error so callers can treat them uniformly.
func IntrospectSchema(label, uri string, ct common.ConnType) (*SchemaCache, error) {
	switch ct {
	case common.Postgres:
		return introspectPostgres(label, uri)
	case common.Mysql:
		return introspectMysql(label, uri)
	case common.Mongo:
		return introspectMongo(label, uri)
	case common.Redis, common.Jq:
		return nil, nil
	}
	return nil, fmt.Errorf("IntrospectSchema: unsupported connection type %q", ct)
}

// sqlColumnRow is the row shape returned by the per-adapter
// information_schema.columns query.
type sqlColumnRow struct {
	Database string
	Table    string
	Column   string
}

// buildSqlDatabases groups (database, table, column) rows into a
// DatabaseSchema slice while preserving first-seen order at every
// level. Pure function — exposed unexported so tests can exercise it
// without a database.
func buildSqlDatabases(rows []sqlColumnRow) []DatabaseSchema {
	type tableAcc struct {
		columns []string
		index   int
	}
	type dbAcc struct {
		tables map[string]*tableAcc
		order  []string
	}
	dbs := map[string]*dbAcc{}
	var dbOrder []string

	for _, r := range rows {
		dba, ok := dbs[r.Database]
		if !ok {
			dba = &dbAcc{tables: map[string]*tableAcc{}}
			dbs[r.Database] = dba
			dbOrder = append(dbOrder, r.Database)
		}
		ta, ok := dba.tables[r.Table]
		if !ok {
			ta = &tableAcc{index: len(dba.order)}
			dba.tables[r.Table] = ta
			dba.order = append(dba.order, r.Table)
		}
		ta.columns = append(ta.columns, r.Column)
	}

	out := make([]DatabaseSchema, 0, len(dbOrder))
	for _, dbName := range dbOrder {
		dba := dbs[dbName]
		tables := make([]TableSchema, 0, len(dba.order))
		for _, tname := range dba.order {
			tables = append(tables, TableSchema{
				Name:    tname,
				Columns: dba.tables[tname].columns,
			})
		}
		out = append(out, DatabaseSchema{Name: dbName, Tables: tables})
	}
	return out
}

const postgresIntrospectQuery = `
SELECT table_schema, table_name, column_name
FROM information_schema.columns
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY table_schema, table_name, ordinal_position
`

const mysqlIntrospectQuery = `
SELECT table_schema, table_name, column_name
FROM information_schema.columns
WHERE table_schema NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')
ORDER BY table_schema, table_name, ordinal_position
`

func introspectPostgres(label, uri string) (*SchemaCache, error) {
	db, err := sql.Open("postgres", uri)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return scanInformationSchema(label, db, postgresIntrospectQuery)
}

func introspectMysql(label, uri string) (*SchemaCache, error) {
	dsn, err := adapters.MysqlDSN(uri)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return scanInformationSchema(label, db, mysqlIntrospectQuery)
}

func scanInformationSchema(label string, db *sql.DB, query string) (*SchemaCache, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collected []sqlColumnRow
	for rows.Next() {
		var r sqlColumnRow
		if err := rows.Scan(&r.Database, &r.Table, &r.Column); err != nil {
			return nil, err
		}
		collected = append(collected, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now := time.Now()
	return &SchemaCache{
		ConnectionLabel:    label,
		PopulatedAt:        &now,
		LastRefreshAttempt: &now,
		Databases:          buildSqlDatabases(collected),
	}, nil
}

func introspectMongo(label, uri string) (*SchemaCache, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoIntrospectTimeout)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	dbNames, err := client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		return nil, err
	}

	var dbs []DatabaseSchema
	for _, dbName := range dbNames {
		if _, skip := systemMongoDatabases[dbName]; skip {
			continue
		}
		db := client.Database(dbName)
		collNames, err := db.ListCollectionNames(ctx, bson.D{})
		if err != nil {
			return nil, err
		}
		sort.Strings(collNames)
		colls := make([]CollectionSchema, 0, len(collNames))
		for _, cn := range collNames {
			fields, ferr := sampleMongoFields(ctx, db.Collection(cn), mongoSampleSize)
			if ferr != nil {
				// Per-collection sampling errors are swallowed so a
				// single bad collection does not derail introspection
				// for the whole database.
				fields = nil
			}
			colls = append(colls, CollectionSchema{Name: cn, Fields: fields})
		}
		dbs = append(dbs, DatabaseSchema{Name: dbName, Collections: colls})
	}

	now := time.Now()
	return &SchemaCache{
		ConnectionLabel:    label,
		PopulatedAt:        &now,
		LastRefreshAttempt: &now,
		Databases:          dbs,
	}, nil
}

// sampleMongoFields returns the union of top-level field names observed
// across up to n recent documents in the collection. Field order is
// alphabetical so suggestion lists are stable across runs.
func sampleMongoFields(ctx context.Context, coll *mongo.Collection, n int64) ([]string, error) {
	cur, err := coll.Find(ctx, bson.D{}, options.Find().SetLimit(n))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	seen := map[string]struct{}{}
	var fields []string
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		for k := range doc {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				fields = append(fields, k)
			}
		}
	}
	if err := cur.Err(); err != nil {
		return fields, err
	}
	sort.Strings(fields)
	return fields, nil
}
