package sql

import "testing"

func TestValidateQueryScope(t *testing.T) {
	tests := []struct {
		name    string
		record  DBConnectionRecord
		sqlText string
		wantErr bool
	}{
		{
			name:    "mysql allows configured database",
			record:  DBConnectionRecord{DBType: "mysql", DatabaseName: "sales"},
			sqlText: "SELECT u.id FROM sales.user u JOIN sales.role r ON r.id = u.role_id",
		},
		{
			name:    "mysql rejects other database in comma join",
			record:  DBConnectionRecord{DBType: "mysql", DatabaseName: "sales"},
			sqlText: "SELECT a.id FROM sales.user a, secret.audit b WHERE a.id = b.user_id",
			wantErr: true,
		},
		{
			name:    "postgres defaults to public",
			record:  DBConnectionRecord{DBType: "postgres", DatabaseName: "app"},
			sqlText: "WITH x AS (SELECT id FROM public.user) SELECT x.id FROM x",
		},
		{
			name:    "postgres rejects other schema in subquery",
			record:  DBConnectionRecord{DBType: "postgres", DatabaseName: "app", ServiceName: "public"},
			sqlText: "SELECT * FROM (SELECT id FROM private.user) x",
			wantErr: true,
		},
		{
			name:    "oracle allows configured schema case insensitively",
			record:  DBConnectionRecord{DBType: "oracle", DatabaseName: "SCOTT"},
			sqlText: `SELECT e."EMPNO" FROM "scott"."EMP" e`,
		},
		{
			name:    "oracle rejects database link",
			record:  DBConnectionRecord{DBType: "oracle", DatabaseName: "SCOTT"},
			sqlText: "SELECT * FROM emp@remote_db",
			wantErr: true,
		},
		{
			name:    "mssql allows schema in current database",
			record:  DBConnectionRecord{DBType: "mssql", DatabaseName: "reporting"},
			sqlText: "SELECT u.id FROM dbo.[user] u",
		},
		{
			name:    "mssql allows explicitly configured database",
			record:  DBConnectionRecord{DBType: "mssql", DatabaseName: "reporting"},
			sqlText: "SELECT u.id FROM reporting.dbo.[user] u",
		},
		{
			name:    "mssql rejects other database",
			record:  DBConnectionRecord{DBType: "mssql", DatabaseName: "reporting"},
			sqlText: "SELECT u.id FROM master.sys.databases u",
			wantErr: true,
		},
		{
			name:    "mssql rejects linked server",
			record:  DBConnectionRecord{DBType: "mssql", DatabaseName: "reporting"},
			sqlText: "SELECT * FROM remote.reporting.dbo.[user]",
			wantErr: true,
		},
		{
			name:    "ignores strings and comments",
			record:  DBConnectionRecord{DBType: "postgres", DatabaseName: "app", ServiceName: "public"},
			sqlText: "SELECT 'FROM private.user' AS sample FROM public.user -- JOIN private.audit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateQueryScope(test.record, test.sqlText)
			if test.wantErr && err == nil {
				t.Fatal("expected scope validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected scope validation error: %v", err)
			}
		})
	}
}
