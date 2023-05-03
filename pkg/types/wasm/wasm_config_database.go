package wasm

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"

	conflog "github.com/machinefi/w3bstream/pkg/depends/conf/log"
	"github.com/machinefi/w3bstream/pkg/depends/conf/postgres"
	"github.com/machinefi/w3bstream/pkg/depends/kit/sqlx"
	"github.com/machinefi/w3bstream/pkg/depends/kit/sqlx/builder"
	"github.com/machinefi/w3bstream/pkg/depends/kit/sqlx/migration"
	"github.com/machinefi/w3bstream/pkg/enums"
	"github.com/machinefi/w3bstream/pkg/types"
)

func NewDatabase(name string) *Database {
	return &Database{Name: name}
}

type Database struct {
	// Name: database name, currently this should be assigned by host; if the
	// database resource can be assigned by project, then open this field.
	Name string `json:"-"`
	// Dialect database dialect, support postgres only now
	Dialect enums.WasmDBDialect `json:"dialect,omitempty,default=''"`
	// Schemas schema list
	Schemas []*Schema `json:"schemas,omitempty"`
	// schemas reference of Schemas; key: schema name
	schemas map[string]*Schema

	ep *postgres.Endpoint // database endpoint
}

type Schema struct {
	// Name: schema name, use postgres driver, default schema is `public`
	Name string `json:"schema,omitempty,default='public'"`
	// Tables: tables define
	Tables []*Table `json:"tables,omitempty"`
}

type Table struct {
	// Name table name
	Name string `json:"name"`
	// Desc table description
	Desc string `json:"desc,omitempty"`
	// Cols table column define
	Cols []*Column `json:"cols"`
	// Keys table index or primary define
	Keys []*Key `json:"keys"`
}

func (t *Table) Build() *builder.Table {
	tbl := builder.T(t.Name)
	tbl.Desc = []string{t.Desc}
	for _, c := range t.Cols {
		tbl.AddCol(c.Build())
	}
	for _, k := range t.Keys {
		tbl.AddKey(k.Build())
	}
	return tbl
}

type Column struct {
	// Name column name
	Name string `json:"name"`
	// Constrains column constrains
	Constrains Constrains `json:"constrains"`
}

func (c Column) Datatype(t enums.WasmDBDatatype) string {
	switch t {
	case
		enums.WASM_DB_DATATYPE__INT,
		enums.WASM_DB_DATATYPE__INT8, enums.WASM_DB_DATATYPE__UINT8,
		enums.WASM_DB_DATATYPE__INT16, enums.WASM_DB_DATATYPE__UINT16,
		enums.WASM_DB_DATATYPE__INT32, enums.WASM_DB_DATATYPE__UINT32,
		enums.WASM_DB_DATATYPE__UINT:
		if c.Constrains.AutoIncrement {
			return "serial"
		} else {
			return "integer"
		}
	case enums.WASM_DB_DATATYPE__INT64, enums.WASM_DB_DATATYPE__UINT64:
		if c.Constrains.AutoIncrement {
			return "bigserial"
		} else {
			return "bigint"
		}
	case enums.WASM_DB_DATATYPE__FLOAT32:
		return "real"
	case enums.WASM_DB_DATATYPE__FLOAT64:
		return "double precision"
	case enums.WASM_DB_DATATYPE__TEXT:
		if c.Constrains.Length < 65536/3 {
			return "character varying"
		} else {
			return "text"
		}
	case enums.WASM_DB_DATATYPE__BOOL:
		return "boolean"
	case enums.WASM_DB_DATATYPE__TIMESTAMP:
		return "bigint"
	case enums.WASM_DB_DATATYPE__DECIMAL:
		return "decimal"
	case enums.WASM_DB_DATATYPE__NUMERIC:
		return "numeric"
	default:
		panic(fmt.Errorf("unsupport type: %v", t.String()))
	}
}

func (c *Column) Build() *builder.Column {
	col := builder.Col(c.Name)
	dt := c.Constrains
	col.ColumnType = &builder.ColumnType{
		DataType:      c.Datatype(c.Constrains.Datatype),
		Length:        dt.Length,
		Decimal:       dt.Decimal,
		Default:       dt.Default,
		Null:          dt.Null,
		AutoIncrement: dt.AutoIncrement,
		Comment:       dt.Desc,
		Desc:          []string{dt.Desc},
	}
	if dt.Default != nil && len(*dt.Default) == 0 {
		*dt.Default = "''"
	}
	return col
}

type Constrains struct {
	Datatype      enums.WasmDBDatatype `json:"datatype"`
	Length        uint64               `json:"length,omitempty"`
	Decimal       uint64               `json:"decimal,omitempty"`
	Default       *string              `json:"default,omitempty"`
	Null          bool                 `json:"null,omitempty"`
	AutoIncrement bool                 `json:"autoincrement,omitempty"`
	Desc          string               `json:"desc,omitempty"`
}

type Key struct {
	Name        string   `json:"name,omitempty"`
	Method      string   `json:"method,omitempty"`
	IsUnique    bool     `json:"isUnique,omitempty"`
	ColumnNames []string `json:"columnNames"`
	Expr        string   `json:"expr,omitempty"`
}

func (k *Key) Build() *builder.Key {
	return &builder.Key{
		Name:     k.Name,
		IsUnique: k.IsUnique,
		Method:   k.Method,
		Def: builder.IndexDef{
			ColNames: k.ColumnNames,
			Expr:     k.Expr,
		},
	}
}
func (d *Database) ConfigType() enums.ConfigType {
	return enums.CONFIG_TYPE__PROJECT_DATABASE
}

func (d *Database) WithContext(ctx context.Context) context.Context {
	return WithSQLStore(ctx, d)
}

func (d *Database) WithSchema(name string) (db sqlx.DBExecutor, err error) {
	if name == "" {
		name = "public"
	}

	if _, ok := d.schemas[name]; !ok {
		return nil, errors.Errorf("schema %s not found in database %s", name, d.Name)
	}
	db = d.ep
	_, err = db.Exec(builder.Expr("SET SEARCH_PATH TO " + name))
	if err != nil {
		return nil, errors.Errorf("switch schema failed: %v", err)
	}
	return db, nil
}

func (d *Database) WithDefaultSchema() (sqlx.DBExecutor, error) {
	return d.WithSchema("public")
}

func (d *Database) Init(ctx context.Context) (err error) {
	// init database endpoint
	d.Name = types.MustProjectFromContext(ctx).DatabaseName()
	d.ep = types.MustWasmDBEndpointFromContext(ctx)
	d.ep.Database = sqlx.NewDatabase(d.Name)
	if d.schemas == nil {
		d.schemas = make(map[string]*Schema)
	}

	// combine schema tables
	if len(d.Schemas) == 0 {
		d.Schemas = append(d.Schemas, &Schema{Name: "public"})
	}
	for _, s := range d.Schemas {
		if s.Name == "" {
			s.Name = "public" // pg default
		}
		if _, ok := d.schemas[s.Name]; !ok {
			d.schemas[s.Name] = &Schema{Name: s.Name}
		}

		d.schemas[s.Name].Tables = append(d.schemas[s.Name].Tables, s.Tables...)
	}

	if err = d.ep.Init(); err != nil {
		return err
	}

	// try to create database before migration
	if _, err = d.ep.Exec(builder.Expr("CREATE DATABASE " + d.Name)); err != nil {
		conflog.Std().Warn(err)
	}

	// init each schema
	for _, s := range d.schemas {
		ep := d.ep
		for _, t := range s.Tables {
			ep.AddTable(t.Build())
		}
		db := ep.WithSchema(s.Name)
		conflog.Std().Info("migrating %s", s.Name)
		if err = migration.Migrate(db, os.Stderr); err != nil {
			conflog.Std().Info(err.Error())
			return err
		}
	}

	return nil
}
