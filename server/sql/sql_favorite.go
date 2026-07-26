package sql

import (
	"errors"
	"strings"

	"db_platform/server/config"
)

/*
sql_favorite.go
----------------------------------------------------------------------
负责当前登录用户 SQL 收藏夹的持久化与连接权限校验。

约束：
1. 每条收藏归属于一个用户。
2. 用户只能查看、编辑和删除自己的收藏。
3. 收藏绑定连接时，必须校验当前用户是否有该连接权限。
4. 收藏列表支持按数据库类型、连接名称和关键字筛选，并优先展示置顶项。
*/

// SQLFavorite 是 SQL 收藏记录模型。
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

// ListSQLFavorites 返回当前用户自己的 SQL 收藏列表。
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
FROM sql_favorite
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
	return items, rows.Err()
}

// CreateSQLFavorite 新增 SQL 收藏。
func CreateSQLFavorite(userID int64, roleName string, item SQLFavorite) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	normalizeFavorite(&item)
	if err := validateFavorite(item); err != nil {
		return err
	}
	if err := ensureFavoriteConnectionAccess(userID, roleName, item.ConnectionName); err != nil {
		return err
	}

	_, err = db.Exec(`
INSERT INTO sql_favorite (
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

// UpdateSQLFavorite 编辑 SQL 收藏。
func UpdateSQLFavorite(userID int64, roleName string, item SQLFavorite) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("收藏 ID 不能为空")
	}
	normalizeFavorite(&item)
	if err := validateFavorite(item); err != nil {
		return err
	}
	if err := ensureFavoriteConnectionAccess(userID, roleName, item.ConnectionName); err != nil {
		return err
	}

	res, err := db.Exec(`
UPDATE sql_favorite
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

// DeleteSQLFavorite 删除当前用户自己的 SQL 收藏。
func DeleteSQLFavorite(userID int64, favoriteID int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if favoriteID <= 0 {
		return errors.New("收藏 ID 不能为空")
	}

	res, err := db.Exec(`
DELETE FROM sql_favorite
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

func normalizeFavorite(item *SQLFavorite) {
	item.FavoriteName = strings.TrimSpace(item.FavoriteName)
	item.SQLText = strings.TrimSpace(item.SQLText)
	item.DBType = strings.TrimSpace(strings.ToLower(item.DBType))
	item.ConnectionName = strings.TrimSpace(item.ConnectionName)
	item.Remark = strings.TrimSpace(item.Remark)
	if item.IsPinned != 0 {
		item.IsPinned = 1
	}
}

func validateFavorite(item SQLFavorite) error {
	if item.FavoriteName == "" {
		return errors.New("收藏名称不能为空")
	}
	if item.SQLText == "" {
		return errors.New("SQL 内容不能为空")
	}
	if item.DBType != "" && item.DBType != "mysql" && item.DBType != "oracle" && item.DBType != "postgres" && item.DBType != "mssql" {
		return errors.New("数据库类型只能是 mysql、oracle、postgres 或 mssql")
	}
	return nil
}

func ensureFavoriteConnectionAccess(userID int64, roleName string, connectionName string) error {
	if connectionName == "" {
		return nil
	}

	ok, err := UserCanAccessConnection(userID, roleName, connectionName)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("当前用户无权使用该数据库连接")
	}
	return nil
}
