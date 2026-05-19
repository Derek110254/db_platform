package sql

import (
	"errors"
	"strings"

	"sql_platform/server/auth"
	"sql_platform/server/config"
)

/*
sql_favorite.go
----------------------------------------------------------------------
该文件专门处理 SQL 收藏功能的增删改查。

设计原则：
1. 每条收藏都归属于某个 user_id
2. 用户只能查看 / 编辑 / 删除自己的收藏
3. 支持按数据库类型、连接名、关键字筛选
4. 支持置顶
5. 使用收藏时，仍然需要在调用查询接口时走数据库连接权限校验
*/

// SQLFavorite
// ----------------------------------------------------------------------
// SQL 收藏结构
type SQLFavorite struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"userId"`
	FavoriteName   string `json:"favoriteName"`
	SQLText        string `json:"sqlText"`
	DBType         string `json:"dbType"`
	ConnectionName string `json:"connectionName"`
	Remark         string `json:"remark"`
	IsPinned       int    `json:"isPinned"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}

// ListSQLFavorites
// ----------------------------------------------------------------------
// 获取当前用户自己的 SQL 收藏列表。
//
// 支持筛选：
// 1. dbType
// 2. connectionName
// 3. keyword（匹配收藏名称、备注、SQL 文本）
func ListSQLFavorites(userID int64, dbType string, connectionName string, keyword string) ([]SQLFavorite, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	dbType = strings.TrimSpace(strings.ToLower(dbType))
	connectionName = strings.TrimSpace(connectionName)
	keyword = strings.TrimSpace(keyword)

	query := `
SELECT id, user_id, favorite_name, sql_text, db_type, connection_name, remark, is_pinned, create_time, update_time
FROM platform_sql_favorite
WHERE user_id = ?
`
	args := []interface{}{userID}

	if dbType != "" {
		query += ` AND db_type = ?`
		args = append(args, dbType)
	}

	if connectionName != "" {
		query += ` AND connection_name = ?`
		args = append(args, connectionName)
	}

	if keyword != "" {
		query += ` AND (favorite_name LIKE ? OR remark LIKE ? OR sql_text LIKE ?)`
		kw := "%" + keyword + "%"
		args = append(args, kw, kw, kw)
	}

	query += ` ORDER BY is_pinned DESC, update_time DESC, id DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]SQLFavorite, 0)
	for rows.Next() {
		var item SQLFavorite
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.FavoriteName,
			&item.SQLText,
			&item.DBType,
			&item.ConnectionName,
			&item.Remark,
			&item.IsPinned,
			&item.CreateTime,
			&item.UpdateTime,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// CreateSQLFavorite
// ----------------------------------------------------------------------
// 新增 SQL 收藏。
//
// 约束：
// 1. 收藏名称不能为空
// 2. SQL 文本不能为空
// 3. 如果填写了 connectionName，则当前用户必须对该连接有权限
func CreateSQLFavorite(userID int64, roleName string, item SQLFavorite) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	item.FavoriteName = strings.TrimSpace(item.FavoriteName)
	item.SQLText = strings.TrimSpace(item.SQLText)
	item.DBType = strings.TrimSpace(strings.ToLower(item.DBType))
	item.ConnectionName = strings.TrimSpace(item.ConnectionName)
	item.Remark = strings.TrimSpace(item.Remark)

	if item.FavoriteName == "" {
		return errors.New("收藏名称不能为空")
	}
	if item.SQLText == "" {
		return errors.New("SQL 内容不能为空")
	}
	if item.DBType != "" && item.DBType != "mysql" && item.DBType != "oracle" {
		return errors.New("数据库类型只能是 mysql 或 oracle")
	}
	if item.IsPinned != 0 {
		item.IsPinned = 1
	}

	// 如果绑定了连接，则校验权限
	if item.ConnectionName != "" {
		ok, err := auth.UserCanAccessConnection(userID, roleName, item.ConnectionName)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("当前用户无权收藏该数据库连接对应 SQL")
		}
	}

	_, err = db.Exec(`
INSERT INTO platform_sql_favorite (
    user_id,
    favorite_name,
    sql_text,
    db_type,
    connection_name,
    remark,
    is_pinned
) VALUES (?, ?, ?, ?, ?, ?, ?)
`, userID, item.FavoriteName, item.SQLText, item.DBType, item.ConnectionName, item.Remark, item.IsPinned)
	return err
}

// UpdateSQLFavorite
// ----------------------------------------------------------------------
// 编辑 SQL 收藏。
//
// 约束：
// 1. 只能编辑自己的收藏
// 2. 如果填写了 connectionName，则必须对该连接有权限
func UpdateSQLFavorite(userID int64, roleName string, item SQLFavorite) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("收藏ID不能为空")
	}

	item.FavoriteName = strings.TrimSpace(item.FavoriteName)
	item.SQLText = strings.TrimSpace(item.SQLText)
	item.DBType = strings.TrimSpace(strings.ToLower(item.DBType))
	item.ConnectionName = strings.TrimSpace(item.ConnectionName)
	item.Remark = strings.TrimSpace(item.Remark)

	if item.FavoriteName == "" {
		return errors.New("收藏名称不能为空")
	}
	if item.SQLText == "" {
		return errors.New("SQL 内容不能为空")
	}
	if item.DBType != "" && item.DBType != "mysql" && item.DBType != "oracle" {
		return errors.New("数据库类型只能是 mysql 或 oracle")
	}
	if item.IsPinned != 0 {
		item.IsPinned = 1
	}

	if item.ConnectionName != "" {
		ok, err := auth.UserCanAccessConnection(userID, roleName, item.ConnectionName)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("当前用户无权使用该数据库连接")
		}
	}

	res, err := db.Exec(`
UPDATE platform_sql_favorite
SET favorite_name = ?, sql_text = ?, db_type = ?, connection_name = ?, remark = ?, is_pinned = ?
WHERE id = ? AND user_id = ?
`, item.FavoriteName, item.SQLText, item.DBType, item.ConnectionName, item.Remark, item.IsPinned, item.ID, userID)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("收藏不存在或无权限编辑")
	}

	return nil
}

// DeleteSQLFavorite
// ----------------------------------------------------------------------
// 删除 SQL 收藏。
// 只能删除自己的收藏。
func DeleteSQLFavorite(userID int64, favoriteID int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if favoriteID <= 0 {
		return errors.New("收藏ID不能为空")
	}

	res, err := db.Exec(`
DELETE FROM platform_sql_favorite
WHERE id = ? AND user_id = ?
`, favoriteID, userID)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("收藏不存在或无权限删除")
	}

	return nil
}
