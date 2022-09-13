// This is a generated source file. DO NOT EDIT
// Source: models/project__generated.go

package models

import (
	"fmt"
	"time"

	"github.com/iotexproject/Bumblebee/base/types"
	"github.com/iotexproject/Bumblebee/kit/sqlx"
	"github.com/iotexproject/Bumblebee/kit/sqlx/builder"
)

var ProjectTable *builder.Table

func init() {
	ProjectTable = DB.Register(&Project{})
}

type ProjectIterator struct {
}

func (ProjectIterator) New() interface{} {
	return &Project{}
}

func (ProjectIterator) Resolve(v interface{}) *Project {
	return v.(*Project)
}

func (*Project) TableName() string {
	return "t_project"
}

func (*Project) TableDesc() []string {
	return []string{
		"Project schema for project information",
	}
}

func (*Project) Comments() map[string]string {
	return map[string]string{}
}

func (*Project) ColDesc() map[string][]string {
	return map[string][]string{}
}

func (*Project) ColRel() map[string][]string {
	return map[string][]string{}
}

func (*Project) PrimaryKey() []string {
	return []string{
		"ID",
	}
}

func (m *Project) IndexFieldNames() []string {
	return []string{
		"ID",
		"Name",
		"Version",
	}
}

func (*Project) UniqueIndexes() builder.Indexes {
	return builder.Indexes{
		"ui_name_version": []string{
			"Name",
			"Version",
			"DeletedAt",
		},
	}
}

func (*Project) UniqueIndexUINameVersion() string {
	return "ui_name_version"
}

func (m *Project) ColID() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldID())
}

func (*Project) FieldID() string {
	return "ID"
}

func (m *Project) ColName() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldName())
}

func (*Project) FieldName() string {
	return "Name"
}

func (m *Project) ColVersion() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldVersion())
}

func (*Project) FieldVersion() string {
	return "Version"
}

func (m *Project) ColProtocol() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldProtocol())
}

func (*Project) FieldProtocol() string {
	return "Protocol"
}

func (m *Project) ColCreatedAt() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldCreatedAt())
}

func (*Project) FieldCreatedAt() string {
	return "CreatedAt"
}

func (m *Project) ColUpdatedAt() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldUpdatedAt())
}

func (*Project) FieldUpdatedAt() string {
	return "UpdatedAt"
}

func (m *Project) ColDeletedAt() *builder.Column {
	return ProjectTable.ColByFieldName(m.FieldDeletedAt())
}

func (*Project) FieldDeletedAt() string {
	return "DeletedAt"
}

func (m *Project) CondByValue(db sqlx.DBExecutor) builder.SqlCondition {
	var (
		tbl  = db.T(m)
		fvs  = builder.FieldValueFromStructByNoneZero(m)
		cond = []builder.SqlCondition{tbl.ColByFieldName("DeletedAt").Eq(0)}
	)

	for _, fn := range m.IndexFieldNames() {
		if v, ok := fvs[fn]; ok {
			cond = append(cond, tbl.ColByFieldName(fn).Eq(v))
			delete(fvs, fn)
		}
	}
	if len(cond) == 0 {
		panic(fmt.Errorf("no field for indexes has value"))
	}
	for fn, v := range fvs {
		cond = append(cond, tbl.ColByFieldName(fn).Eq(v))
	}
	return builder.And(cond...)
}

func (m *Project) Create(db sqlx.DBExecutor) error {

	if m.CreatedAt.IsZero() {
		m.CreatedAt.Set(time.Now())
	}

	if m.UpdatedAt.IsZero() {
		m.UpdatedAt.Set(time.Now())
	}

	_, err := db.Exec(sqlx.InsertToDB(db, m, nil))
	return err
}

func (m *Project) List(db sqlx.DBExecutor, cond builder.SqlCondition, adds ...builder.Addition) ([]Project, error) {
	var (
		tbl = db.T(m)
		lst = make([]Project, 0)
	)
	cond = builder.And(tbl.ColByFieldName("DeletedAt").Eq(0), cond)
	adds = append([]builder.Addition{builder.Where(cond), builder.Comment("Project.List")}, adds...)
	err := db.QueryAndScan(builder.Select(nil).From(tbl, adds...), &lst)
	return lst, err
}

func (m *Project) Count(db sqlx.DBExecutor, cond builder.SqlCondition, adds ...builder.Addition) (cnt int64, err error) {
	tbl := db.T(m)
	cond = builder.And(tbl.ColByFieldName("DeletedAt").Eq(0), cond)
	adds = append([]builder.Addition{builder.Where(cond), builder.Comment("Project.List")}, adds...)
	err = db.QueryAndScan(builder.Select(builder.Count()).From(tbl, adds...), &cnt)
	return
}

func (m *Project) FetchByID(db sqlx.DBExecutor) error {
	tbl := db.T(m)
	err := db.QueryAndScan(
		builder.Select(nil).
			From(
				tbl,
				builder.Where(
					builder.And(
						tbl.ColByFieldName("ID").Eq(m.ID),
						tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
					),
				),
				builder.Comment("Project.FetchByID"),
			),
		m,
	)
	return err
}

func (m *Project) FetchByNameAndVersion(db sqlx.DBExecutor) error {
	tbl := db.T(m)
	err := db.QueryAndScan(
		builder.Select(nil).
			From(
				tbl,
				builder.Where(
					builder.And(
						tbl.ColByFieldName("Name").Eq(m.Name),
						tbl.ColByFieldName("Version").Eq(m.Version),
						tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
					),
				),
				builder.Comment("Project.FetchByNameAndVersion"),
			),
		m,
	)
	return err
}

func (m *Project) UpdateByIDWithFVs(db sqlx.DBExecutor, fvs builder.FieldValues) error {

	if _, ok := fvs["UpdatedAt"]; !ok {
		fvs["UpdatedAt"] = types.Timestamp{Time: time.Now()}
	}
	tbl := db.T(m)
	res, err := db.Exec(
		builder.Update(tbl).
			Where(
				builder.And(
					tbl.ColByFieldName("ID").Eq(m.ID),
					tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
				),
				builder.Comment("Project.UpdateByIDWithFVs"),
			).
			Set(tbl.AssignmentsByFieldValues(fvs)...),
	)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return m.FetchByID(db)
	}
	return nil
}

func (m *Project) UpdateByID(db sqlx.DBExecutor, zeros ...string) error {
	fvs := builder.FieldValueFromStructByNoneZero(m, zeros...)
	return m.UpdateByIDWithFVs(db, fvs)
}

func (m *Project) UpdateByNameAndVersionWithFVs(db sqlx.DBExecutor, fvs builder.FieldValues) error {

	if _, ok := fvs["UpdatedAt"]; !ok {
		fvs["UpdatedAt"] = types.Timestamp{Time: time.Now()}
	}
	tbl := db.T(m)
	res, err := db.Exec(
		builder.Update(tbl).
			Where(
				builder.And(
					tbl.ColByFieldName("Name").Eq(m.Name),
					tbl.ColByFieldName("Version").Eq(m.Version),
					tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
				),
				builder.Comment("Project.UpdateByNameAndVersionWithFVs"),
			).
			Set(tbl.AssignmentsByFieldValues(fvs)...),
	)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return m.FetchByNameAndVersion(db)
	}
	return nil
}

func (m *Project) UpdateByNameAndVersion(db sqlx.DBExecutor, zeros ...string) error {
	fvs := builder.FieldValueFromStructByNoneZero(m, zeros...)
	return m.UpdateByNameAndVersionWithFVs(db, fvs)
}

func (m *Project) Delete(db sqlx.DBExecutor) error {
	_, err := db.Exec(
		builder.Delete().
			From(
				db.T(m),
				builder.Where(m.CondByValue(db)),
				builder.Comment("Project.Delete"),
			),
	)
	return err
}

func (m *Project) DeleteByID(db sqlx.DBExecutor) error {
	tbl := db.T(m)
	_, err := db.Exec(
		builder.Delete().
			From(
				tbl,
				builder.Where(
					builder.And(
						tbl.ColByFieldName("ID").Eq(m.ID),
						tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
					),
				),
				builder.Comment("Project.DeleteByID"),
			),
	)
	return err
}

func (m *Project) SoftDeleteByID(db sqlx.DBExecutor) error {
	tbl := db.T(m)
	fvs := builder.FieldValues{}

	if _, ok := fvs["DeletedAt"]; !ok {
		fvs["DeletedAt"] = types.Timestamp{Time: time.Now()}
	}

	if _, ok := fvs["UpdatedAt"]; !ok {
		fvs["UpdatedAt"] = types.Timestamp{Time: time.Now()}
	}
	_, err := db.Exec(
		builder.Update(db.T(m)).
			Where(
				builder.And(
					tbl.ColByFieldName("ID").Eq(m.ID),
					tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
				),
				builder.Comment("Project.SoftDeleteByID"),
			).
			Set(tbl.AssignmentsByFieldValues(fvs)...),
	)
	return err
}

func (m *Project) DeleteByNameAndVersion(db sqlx.DBExecutor) error {
	tbl := db.T(m)
	_, err := db.Exec(
		builder.Delete().
			From(
				tbl,
				builder.Where(
					builder.And(
						tbl.ColByFieldName("Name").Eq(m.Name),
						tbl.ColByFieldName("Version").Eq(m.Version),
						tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
					),
				),
				builder.Comment("Project.DeleteByNameAndVersion"),
			),
	)
	return err
}

func (m *Project) SoftDeleteByNameAndVersion(db sqlx.DBExecutor) error {
	tbl := db.T(m)
	fvs := builder.FieldValues{}

	if _, ok := fvs["DeletedAt"]; !ok {
		fvs["DeletedAt"] = types.Timestamp{Time: time.Now()}
	}

	if _, ok := fvs["UpdatedAt"]; !ok {
		fvs["UpdatedAt"] = types.Timestamp{Time: time.Now()}
	}
	_, err := db.Exec(
		builder.Update(db.T(m)).
			Where(
				builder.And(
					tbl.ColByFieldName("Name").Eq(m.Name),
					tbl.ColByFieldName("Version").Eq(m.Version),
					tbl.ColByFieldName("DeletedAt").Eq(m.DeletedAt),
				),
				builder.Comment("Project.SoftDeleteByNameAndVersion"),
			).
			Set(tbl.AssignmentsByFieldValues(fvs)...),
	)
	return err
}