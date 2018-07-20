// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/charset"
)

func defaultValueToSQL(opt *ast.ColumnOption) string {
	sql := " DEFAULT "
	datum := opt.Expr.GetDatum()
	switch datum.Kind() {
	case types.KindNull:
		expr, ok := opt.Expr.(*ast.FuncCallExpr)
		if ok {
			sql += expr.FnName.O
		} else {
			sql += "NULL"
		}

	case types.KindInt64:
		sql += strconv.FormatInt(datum.GetInt64(), 10)

	case types.KindString:
		sql += formatStringValue(datum.GetString())

	default:
		sql += fmt.Sprintf("%v", datum.GetValue())
	}

	return sql
}

func formatStringValue(s string) string {
	if s == "" {
		return "''"
	}
	return fmt.Sprintf("'%s'", s)
}

// for change/modify column use only.
func fieldTypeToSQL(ft *types.FieldType) string {
	strs := []string{ft.CompactStr()}
	log.Debugf("tp %v, flag %v, flen %v, decimal %v, charset %v, collate %v, strs %v",
		ft.Tp, ft.Flag, ft.Flen, ft.Decimal, ft.Charset, ft.Collate, strs)
	if mysql.HasUnsignedFlag(ft.Flag) {
		strs = append(strs, "UNSIGNED")
	}
	if mysql.HasZerofillFlag(ft.Flag) {
		strs = append(strs, "ZEROFILL")
	}

	if mysql.HasBinaryFlag(ft.Flag) && (ft.Charset != charset.CharsetBin || (!types.IsTypeChar(ft.Tp) && !types.IsTypeBlob(ft.Tp))) {
		strs = append(strs, "BINARY")
	}

	return strings.Join(strs, " ")
}

// for add column use only. Work likes FieldType.String(), but bug free(?)
func fullFieldTypeToSQL(ft *types.FieldType) string {
	sql := fieldTypeToSQL(ft)
	strs := strings.Split(sql, " ")

	if types.IsTypeChar(ft.Tp) || types.IsTypeBlob(ft.Tp) {
		if ft.Charset != "" && ft.Charset != charset.CharsetBin {
			strs = append(strs, fmt.Sprintf("CHARACTER SET %s", ft.Charset))
		}
		if ft.Collate != "" && ft.Collate != charset.CharsetBin {
			strs = append(strs, fmt.Sprintf("COLLATE %s", ft.Collate))
		}
	}

	return strings.Join(strs, " ")
}

// FIXME: tidb's AST is error-some to handle more condition
func columnOptionsToSQL(options []*ast.ColumnOption) string {
	sql := ""
	for _, opt := range options {
		switch opt.Tp {
		case ast.ColumnOptionNotNull:
			sql += " NOT NULL"
		case ast.ColumnOptionNull:
			sql += " NULL"
		case ast.ColumnOptionDefaultValue:
			sql += defaultValueToSQL(opt)
		case ast.ColumnOptionAutoIncrement:
			sql += " AUTO_INCREMENT"
		case ast.ColumnOptionUniqKey:
			sql += " UNIQUE KEY"
		case ast.ColumnOptionPrimaryKey:
			sql += " PRIMARY KEY"
		case ast.ColumnOptionComment:
			sql += fmt.Sprintf(" COMMENT '%v'", opt.Expr.GetValue())
		case ast.ColumnOptionOnUpdate: // For Timestamp and Datetime only.
			sql += " ON UPDATE CURRENT_TIMESTAMP"
		case ast.ColumnOptionFulltext:
			panic("not implemented yet")
		default:
			panic("not implemented yet")
		}
	}

	return sql
}

func escapeName(name string) string {
	return strings.Replace(name, "`", "``", -1)
}

func tableNameToSQL(tbl *ast.TableName) string {
	sql := ""
	if tbl.Schema.O != "" {
		sql += fmt.Sprintf("`%s`.", tbl.Schema.O)
	}
	sql += fmt.Sprintf("`%s`", tbl.Name.O)
	return sql
}

func columnNameToSQL(name *ast.ColumnName) string {
	sql := ""
	if name.Schema.O != "" {
		sql += fmt.Sprintf("`%s`.", escapeName(name.Schema.O))
	}
	if name.Table.O != "" {
		sql += fmt.Sprintf("`%s`.", escapeName(name.Table.O))
	}
	sql += fmt.Sprintf("`%s`", escapeName(name.Name.O))
	return sql
}

func indexColNameToSQL(name *ast.IndexColName) string {
	sql := columnNameToSQL(name.Column)
	if name.Length != types.UnspecifiedLength {
		sql += fmt.Sprintf(" (%d)", name.Length)
	}
	return sql
}

func constraintKeysToSQL(keys []*ast.IndexColName) string {
	if len(keys) == 0 {
		panic("unreachable")
	}
	sql := ""
	for i, indexColName := range keys {
		if i == 0 {
			sql += "("
		}
		sql += indexColNameToSQL(indexColName)
		if i != len(keys)-1 {
			sql += ", "
		}
	}
	sql += ")"
	return sql
}

func referenceDefToSQL(refer *ast.ReferenceDef) string {
	sql := fmt.Sprintf("%s ", tableNameToSQL(refer.Table))
	sql += constraintKeysToSQL(refer.IndexColNames)
	if refer.OnDelete != nil && refer.OnDelete.ReferOpt != ast.ReferOptionNoOption {
		sql += fmt.Sprintf(" ON DELETE %s", refer.OnDelete.ReferOpt)
	}
	if refer.OnUpdate != nil && refer.OnUpdate.ReferOpt != ast.ReferOptionNoOption {
		sql += fmt.Sprintf(" ON UPDATE %s", refer.OnUpdate.ReferOpt)
	}
	return sql
}

func indexTypeToSQL(opt *ast.IndexOption) string {
	// opt can be nil.
	if opt == nil {
		return ""
	}
	switch opt.Tp {
	case model.IndexTypeBtree:
		return "USING BTREE "
	case model.IndexTypeHash:
		return "USING HASH "
	default:
		// nothing to do
		return ""
	}
}

func constraintToSQL(constraint *ast.Constraint) string {
	sql := ""
	switch constraint.Tp {
	case ast.ConstraintKey, ast.ConstraintIndex:
		sql += "ADD INDEX "
		if constraint.Name != "" {
			sql += fmt.Sprintf("`%s` ", escapeName(constraint.Name))
		}
		sql += indexTypeToSQL(constraint.Option)
		sql += constraintKeysToSQL(constraint.Keys)
		sql += indexOptionToSQL(constraint.Option)

	case ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex:
		sql += "ADD CONSTRAINT "
		if constraint.Name != "" {
			sql += fmt.Sprintf("`%s` ", escapeName(constraint.Name))
		}
		sql += "UNIQUE INDEX "
		sql += indexTypeToSQL(constraint.Option)
		sql += constraintKeysToSQL(constraint.Keys)
		sql += indexOptionToSQL(constraint.Option)

	case ast.ConstraintForeignKey:
		sql += "ADD CONSTRAINT "
		if constraint.Name != "" {
			sql += fmt.Sprintf("`%s` ", escapeName(constraint.Name))
		}
		sql += "FOREIGN KEY "
		sql += constraintKeysToSQL(constraint.Keys)
		sql += " REFERENCES "
		sql += referenceDefToSQL(constraint.Refer)

	case ast.ConstraintPrimaryKey:
		sql += "ADD CONSTRAINT "
		if constraint.Name != "" {
			sql += fmt.Sprintf("`%s` ", escapeName(constraint.Name))
		}
		sql += "PRIMARY KEY "
		sql += indexTypeToSQL(constraint.Option)
		sql += constraintKeysToSQL(constraint.Keys)
		sql += indexOptionToSQL(constraint.Option)

	case ast.ConstraintFulltext:
		sql += "ADD FULLTEXT INDEX "
		if constraint.Name != "" {
			sql += fmt.Sprintf("`%s` ", escapeName(constraint.Name))
		}
		sql += constraintKeysToSQL(constraint.Keys)
		sql += indexOptionToSQL(constraint.Option)

	default:
		panic("not implemented yet")
	}
	return sql
}

func positionToSQL(pos *ast.ColumnPosition) string {
	var sql string
	switch pos.Tp {
	case ast.ColumnPositionNone:
	case ast.ColumnPositionFirst:
		sql = " FIRST"
	case ast.ColumnPositionAfter:
		colName := pos.RelativeColumn.Name.O
		sql = fmt.Sprintf(" AFTER `%s`", escapeName(colName))
	default:
		panic("unreachable")
	}
	return sql
}

// Convert constraint indexoption to sql. Currently only support comment.
func indexOptionToSQL(option *ast.IndexOption) string {
	if option == nil {
		return ""
	}

	if option.Comment != "" {
		return fmt.Sprintf(" COMMENT '%s'", option.Comment)
	}

	return ""
}

func tableOptionsToSQL(options []*ast.TableOption) string {
	if len(options) == 0 {
		return ""
	}

	sqls := make([]string, 0, len(options))
	for _, opt := range options {
		sql, err := AnalyzeTableOption(opt)
		if err != nil {
			panic(err) // refine it later
		}

		sqls = append(sqls, sql)
	}

	return strings.Join(sqls, " ")
}

func formatRowFormat(rf uint64) string {
	var s string
	switch rf {
	case ast.RowFormatDefault:
		s = "DEFAULT"
	case ast.RowFormatDynamic:
		s = "DYNAMIC"
	case ast.RowFormatFixed:
		s = "FIXED"
	case ast.RowFormatCompressed:
		s = "COMPRESSED"
	case ast.RowFormatRedundant:
		s = "REDUNDANT"
	case ast.RowFormatCompact:
		s = "COMPACT"
	default:
		panic("unreachable")
	}
	return s
}

func columnToSQL(typeDef string, newColumn *ast.ColumnDef) string {
	return fmt.Sprintf("%s %s%s", columnNameToSQL(newColumn.Name), typeDef, columnOptionsToSQL(newColumn.Options))
}

func alterTableSpecToSQL(spec *ast.AlterTableSpec) string {
	var (
		suffixes []string
		suffix   string
	)

	switch spec.Tp {
	case ast.AlterTableOption:
		return tableOptionsToSQL(spec.Options)

	case ast.AlterTableAddColumns:
		for _, newColumn := range spec.NewColumns {
			typeDef := fullFieldTypeToSQL(newColumn.Tp)
			suffix = columnToSQL(typeDef, newColumn)
			if spec.Position != nil {
				suffix += positionToSQL(spec.Position)
			}
			suffixes = append(suffixes, suffix)
		}
		return fmt.Sprintf("ADD COLUMN (%s)", strings.Join(suffixes, ","))

	case ast.AlterTableDropColumn:
		return fmt.Sprintf("DROP COLUMN %s", columnNameToSQL(spec.OldColumnName))

	case ast.AlterTableDropIndex:
		return fmt.Sprintf("DROP INDEX `%s`", escapeName(spec.Name))

	case ast.AlterTableAddConstraint:
		return constraintToSQL(spec.Constraint)

	case ast.AlterTableDropForeignKey:
		return fmt.Sprintf("DROP FOREIGN KEY `%s`", escapeName(spec.Name))

	case ast.AlterTableModifyColumn:
		// TiDB doesn't support alter table modify column charset and collation.
		typeDef := fieldTypeToSQL(spec.NewColumns[0].Tp)
		suffix += fmt.Sprintf("MODIFY COLUMN %s", columnToSQL(typeDef, spec.NewColumns[0]))
		if spec.Position != nil {
			suffix += positionToSQL(spec.Position)
		}
		return suffix

	// FIXME: should support [FIRST|AFTER col_name], but tidb parser not support this currently.
	case ast.AlterTableChangeColumn:
		// TiDB doesn't support alter table change column charset and collation.
		typeDef := fieldTypeToSQL(spec.NewColumns[0].Tp)
		suffix += "CHANGE COLUMN "
		suffix += fmt.Sprintf("%s %s",
			columnNameToSQL(spec.OldColumnName),
			columnToSQL(typeDef, spec.NewColumns[0]))
		if spec.Position != nil {
			suffix += positionToSQL(spec.Position)
		}
		return suffix

	case ast.AlterTableRenameTable:
		return fmt.Sprintf("RENAME TO %s", tableNameToSQL(spec.NewTable))

	case ast.AlterTableAlterColumn:
		suffix += fmt.Sprintf("ALTER COLUMN %s ", columnNameToSQL(spec.NewColumns[0].Name))
		if options := spec.NewColumns[0].Options; options != nil {
			suffix += fmt.Sprintf("SET DEFAULT %v", options[0].Expr.GetValue())
		} else {
			suffix += "DROP DEFAULT"
		}
		return suffix

	case ast.AlterTableDropPrimaryKey:
		return "DROP PRIMARY KEY"

	case ast.AlterTableLock:
		// just ignore it
	}

	return ""
}

func alterTableStmtToSQL(stmt *ast.AlterTableStmt) string {
	var (
		defStrs = make([]string, 0, len(stmt.Specs))
		prefix  = fmt.Sprintf("ALTER TABLE %s ", tableNameToSQL(stmt.Table))
	)
	for _, spec := range stmt.Specs {
		defStr := alterTableSpecToSQL(spec)
		log.Infof("spec %+v, text %s", spec, defStr)
		if len(defStr) == 0 {
			continue
		}

		defStrs = append(defStrs, defStr)
	}
	query := fmt.Sprintf("%s %s", prefix, strings.Join(defStrs, ","))
	log.Debugf("alter table stmt to query %s", query)

	return query
}