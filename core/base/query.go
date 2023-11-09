package base

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/baowk/dilu-core/common/utils"
)

/*
*  条件查询结构体，结果体非零值字段将查询
*  @Param type
* 	eq  等于(默认不填都可以)
* 	like  包含
*	gt / gte 大于 / 大于等于
*	lt / lte 小于 / 小于等于
*	left  / ileft ：like xxx%
*	right / iright  : like %xxx
*	in
*	isnull
*  	order 排序		e.g. order[key]=desc     order[key]=asc
*   "-" 忽略该字段
*  @Param table
*  	table 不填默认取 TableName值
*  @Param column
*  	column 不填以结构体字段
*  eg：
*  type ExampleQuery struct{
*  	Name     string `json:"name" query:"type:like;column:name;table:exampale"`
* 		Status   int    `json:"status" query:"type:gt"`
*  }
*  func (ExampleQuery) TableName() string {
*		return "ExampleQuery"
*  }
 */
type Query interface {
	TableName() string
}

const (
	// FromQueryTag tag标记
	FromQueryTag = "query"
	// Mysql 数据库标识
	Mysql = "mysql"
	// Postgres 数据库标识
	Postgres = "postgres"
)

// ResolveSearchQuery 解析
/**
 * 	eq  等于(默认不填都可以)
 * 	like  包含
 *	gt / gte 大于 / 大于等于
 *	lt / lte 小于 / 小于等于
 *	left  / ileft ：like xxx%
 *	right / iright  : like %xxx
 *	in
 *	isnull
 *  order 排序		e.g. order[key]=desc     order[key]=asc
 */
func ResolveSearchQuery(driver string, q any, condition Condition, pTName string) {
	qType := reflect.TypeOf(q)
	qValue := reflect.ValueOf(q)
	var tag string
	var ok bool
	var t *resolveSearchTag
	var tname string
	if cur, ok := q.(Query); ok {
		if cur.TableName() == "" {
			tname = pTName
		} else {
			tname = cur.TableName()
		}
	} else {
		tname = pTName
	}

	for i := 0; i < qType.NumField(); i++ {
		tag, ok = "", false
		tag, ok = qType.Field(i).Tag.Lookup(FromQueryTag)
		if !ok {
			//递归调用
			ResolveSearchQuery(driver, qValue.Field(i).Interface(), condition, tname)
			continue
		}
		switch tag {
		case "-":
			continue
		}
		if qValue.Field(i).IsZero() {
			continue
		}
		t = makeTag(tag)
		if t.Column == "" {
			t.Column = utils.SnakeCase(qType.Field(i).Name, false)
		}
		if t.Table == "" {
			t.Table = tname
		}

		//解析 Postgres `语法不支持，单独适配
		if driver == Postgres {
			pgSql(driver, t, condition, qValue, i, tname)
		} else {
			otherSql(driver, t, condition, qValue, i, tname)
		}
	}
}

type QueryTag string

const (
	EQ     QueryTag = "eq"
	LIKE   QueryTag = "like"
	ILIKE  QueryTag = "ilike"
	LEFT   QueryTag = "left"
	ILEFT  QueryTag = "ileft"
	RIGHT  QueryTag = "right"
	IRIGHT QueryTag = "iright"
	GT     QueryTag = "gt"
	GTE    QueryTag = "gte"
	LT     QueryTag = "lt"
	LTE    QueryTag = "lte"
	IN     QueryTag = "in"
	ISNULL QueryTag = "isnull"
	ORDER  QueryTag = "order"
	JOIN   QueryTag = "join"
)

func pgSql(driver string, t *resolveSearchTag, condition Condition, qValue reflect.Value, i int, tname string) {
	if t.Type == "" {
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		return
	}
	qtag := QueryTag(t.Type)
	switch qtag {
	case EQ:
		condition.SetWhere(fmt.Sprintf("%s.%s = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case ILIKE:
		condition.SetWhere(fmt.Sprintf("%s.%s ilike ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String() + "%"})
		break
	case LIKE:
		condition.SetWhere(fmt.Sprintf("%s.%s like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String() + "%"})
		break
	case GT:
		condition.SetWhere(fmt.Sprintf("%s.%s > ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case GTE:
		condition.SetWhere(fmt.Sprintf("%s.%s >= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case LT:
		condition.SetWhere(fmt.Sprintf("%s.%s < ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case LTE:
		condition.SetWhere(fmt.Sprintf("%s.%s <= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case ILEFT:
		condition.SetWhere(fmt.Sprintf("%s.%s ilike ?", t.Table, t.Column), []interface{}{qValue.Field(i).String() + "%"})
		break
	case LEFT:
		condition.SetWhere(fmt.Sprintf("%s.%s like ?", t.Table, t.Column), []interface{}{qValue.Field(i).String() + "%"})
		break
	case IRIGHT:
		condition.SetWhere(fmt.Sprintf("%s.%s ilike ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String()})
		break
	case RIGHT:
		condition.SetWhere(fmt.Sprintf("%s.%s like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String()})
		break
	case IN:
		condition.SetWhere(fmt.Sprintf("%s.%s in (?)", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case ISNULL:
		if !(qValue.Field(i).IsZero() && qValue.Field(i).IsNil()) {
			condition.SetWhere(fmt.Sprintf("%s.%s isnull", t.Table, t.Column), make([]interface{}, 0))
		}
		break
	case ORDER:
		switch strings.ToLower(qValue.Field(i).String()) {
		case "desc", "asc":
			condition.SetOrder(fmt.Sprintf("%s.%s %s", t.Table, t.Column, qValue.Field(i).String()))
		}
		break
	case JOIN:
		//左关联
		join := condition.SetJoinOn(t.Type, fmt.Sprintf(
			"left join %s on %s.%s = %s.%s", t.Join, t.Join, t.On[0], t.Table, t.On[1],
		))
		ResolveSearchQuery(driver, qValue.Field(i).Interface(), join, tname)
		break
	default:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	}
}

func otherSql(driver string, t *resolveSearchTag, condition Condition, qValue reflect.Value, i int, tname string) {
	if t.Type == "" {
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		return
	}
	qtag := QueryTag(t.Type)
	switch qtag {
	case EQ:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case GT:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` > ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case GTE:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` >= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case LT:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` < ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case LTE:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` <= ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case LEFT:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` like ?", t.Table, t.Column), []interface{}{qValue.Field(i).String() + "%"})
		break
	case LIKE:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String() + "%"})
		break
	case RIGHT:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` like ?", t.Table, t.Column), []interface{}{"%" + qValue.Field(i).String()})
		break
	case IN:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` in (?)", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
		break
	case ISNULL:
		if !(qValue.Field(i).IsZero() && qValue.Field(i).IsNil()) {
			condition.SetWhere(fmt.Sprintf("`%s`.`%s` isnull", t.Table, t.Column), make([]interface{}, 0))
		}
		break
	case ORDER:
		switch strings.ToLower(qValue.Field(i).String()) {
		case "desc", "asc":
			condition.SetOrder(fmt.Sprintf("`%s`.`%s` %s", t.Table, t.Column, qValue.Field(i).String()))
		}
		break
	case JOIN:
		//左关联
		join := condition.SetJoinOn(t.Type, fmt.Sprintf(
			"left join `%s` on `%s`.`%s` = `%s`.`%s`",
			t.Join,
			t.Join,
			t.On[0],
			t.Table,
			t.On[1],
		))
		ResolveSearchQuery(driver, qValue.Field(i).Interface(), join, tname)
		break
	default:
		condition.SetWhere(fmt.Sprintf("`%s`.`%s` = ?", t.Table, t.Column), []interface{}{qValue.Field(i).Interface()})
	}
}