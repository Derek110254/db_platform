<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  autocompletion,
  type Completion,
  type CompletionContext,
} from "@codemirror/autocomplete";
import { sql } from "@codemirror/lang-sql";
import { Prec } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import {
  BookmarkIcon,
  Columns3Icon,
  DatabaseIcon,
  DownloadIcon,
  EraserIcon,
  HistoryIcon,
  KeyRoundIcon,
  LogOutIcon,
  Maximize2Icon,
  Minimize2Icon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  PanelRightCloseIcon,
  PanelRightOpenIcon,
  PencilIcon,
  PlayIcon,
  PlusIcon,
  RefreshCwIcon,
  SaveIcon,
  SearchIcon,
  ServerCogIcon,
  ShieldCheckIcon,
  StarIcon,
  Table2Icon,
  Trash2Icon,
  UserIcon,
  UsersIcon,
  XIcon,
} from "@lucide/vue";
import { Codemirror } from "vue-codemirror";

// 页面只保留查询工作台、数据库连接管理、用户权限三类视图。
type Page = "query" | "connections" | "users";
type DBType = "mysql" | "oracle" | "postgres" | "mssql";

// 当前登录用户信息由后端会话接口返回。
interface CurrentUser {
  userId: number;
  username: string;
  displayName: string;
  role: string;
  canQueryData: number;
  needChangePwd: number;
}

interface QueryConnection {
  name: string;
  dbType: DBType;
  host: string;
  port: number;
  database: string;
  serviceName: string;
  canConnect: number;
}

interface AdminConnection {
  id: number;
  name: string;
  dbType: DBType;
  host: string;
  port: number;
  username: string;
  password?: string;
  databaseName: string;
  serviceName: string;
  isEnabled: number;
  canConnect: number;
}

interface UserItem {
  id: number;
  username: string;
  displayName: string;
  roleName: string;
  isEnabled: number;
  canQueryData: number;
  allowedConnections: string[];
}

interface MetadataTable {
  name: string;
  comment: string;
}

interface MetadataColumn {
  tableName: string;
  columnName: string;
  columnType: string;
  comment: string;
}

interface FavoriteItem {
  id: number;
  favoriteName: string;
  sqlText: string;
  dbType: DBType;
  connectionName: string;
  remark: string;
  isPinned: number;
}

interface QueryHistoryItem {
  id: string;
  connectionName: string;
  dbType: DBType;
  sql: string;
  createdAt: string;
}

// 查询历史按用户保存在浏览器本地，仅保留最近 20 条有返回数据的记录。
const HISTORY_KEY = "db_query_history_v1";
const MAX_HISTORY_ITEMS = 20;

// 登录态和密码修改弹窗状态。
const currentUser = ref<CurrentUser | null>(null);
const authChecking = ref(true);
const loginLoading = ref(false);
const loginMessage = ref("");
const loginForm = ref({ username: "", password: "" });
const changePwdVisible = ref(false);
const changePwdForm = ref({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});
const changePwdMessage = ref("");

// 顶层页面状态和通用提示。
const page = ref<Page>("query");
const message = ref("");
const sidebarCollapsed = ref(false);

// 查询工作台状态：连接、SQL、结果分页、元数据和本地历史。
const dbType = ref<DBType>("mysql");
const connections = ref<QueryConnection[]>([]);
const selectedConnectionName = ref("");
const queryText = ref("");
const queryLoading = ref(false);
const exportLoading = ref(false);
const queryMessage = ref("请输入 SELECT 或 WITH 查询语句");
const columns = ref<string[]>([]);
const rows = ref<Record<string, unknown>[]>([]);
const currentPageNo = ref(1);
const pageSize = ref(20);
const pageSizeOptions = [20, 50, 100];
const resultVisible = ref(false);
const resultMaximized = ref(false);
const resultHeight = ref(520);
const metadataCollapsed = ref(false);
const metadataKeyword = ref("");
const metadataTables = ref<MetadataTable[]>([]);
const metadataColumns = ref<MetadataColumn[]>([]);
const selectedTable = ref("");
let metadataSearchTimer: number | undefined;
const history = ref<QueryHistoryItem[]>([]);

// 历史/收藏抽屉和 SQL 收藏编辑表单。
const favoritesVisible = ref(false);
const drawerTab = ref<"history" | "favorites">("history");
const favorites = ref<FavoriteItem[]>([]);
const favoriteForm = ref({
  id: 0,
  favoriteName: "",
  sqlText: "",
  dbType: "mysql" as DBType,
  connectionName: "",
  remark: "",
  isPinned: 0,
});

// 管理员维护数据库连接时使用的表单状态。
const adminConnections = ref<AdminConnection[]>([]);
const connectionFormVisible = ref(false);
const connectionFormMessage = ref("");
const connectionForm = ref<AdminConnection>({
  id: 0,
  name: "",
  dbType: "mysql",
  host: "",
  port: 3306,
  username: "",
  password: "",
  databaseName: "",
  serviceName: "",
  isEnabled: 1,
  canConnect: 1,
});

// 管理员维护用户和连接授权时使用的表单状态。
const users = ref<UserItem[]>([]);
const userFormVisible = ref(false);
const userForm = ref({
  id: 0,
  username: "",
  password: "",
  displayName: "",
  roleName: "user",
  isEnabled: 1,
  canQueryData: 1,
  allowedConnections: [] as string[],
});

// 派生状态集中描述当前权限、可选连接和结果分页。
const isAuthenticated = computed(() => !!currentUser.value);
const isAdmin = computed(() => currentUser.value?.role === "admin");
const historyStorageKey = computed(() => {
  const userKey =
    currentUser.value?.userId || currentUser.value?.username || "guest";
  return `${HISTORY_KEY}_${userKey}`;
});
const pageTitle = computed(() =>
  page.value === "query"
    ? "查询工作台"
    : page.value === "connections"
      ? "数据库连接管理"
      : "用户与权限",
);
const filteredConnections = computed(() =>
  connections.value.filter((item) => item.dbType === dbType.value),
);
const selectedConnection = computed(() =>
  filteredConnections.value.find(
    (item) => item.name === selectedConnectionName.value,
  ),
);
const canQuery = computed(() => selectedConnection.value?.canConnect !== 0);
const connectionPermissionHint = computed(() => {
  if (connectionForm.value.dbType === "oracle") {
    return "Oracle 元数据依赖 ALL_TABLES / ALL_TAB_COLUMNS。只读用户查询其他 schema 时，需要目标 schema 授权 SELECT，例如：GRANT SELECT ON 目标SCHEMA.表名 TO 只读用户。";
  }
  if (connectionForm.value.dbType === "postgres") {
    return "PostgreSQL 若指定 schema，需要给只读用户授权 USAGE ON SCHEMA 和表 SELECT，例如：GRANT USAGE ON SCHEMA schema名 TO 用户；GRANT SELECT ON ALL TABLES IN SCHEMA schema名 TO 用户。";
  }
  if (connectionForm.value.dbType === "mssql") {
    return "MSSQL 元数据依赖 sys.objects / sys.columns。若表能查但元数据不完整，可授予目标库 VIEW DEFINITION；查询数据仍需表 SELECT 权限。";
  }
  return "MySQL 元数据来自 information_schema。若元数据或查询结果为空，请确认账号拥有目标库表的 SELECT 权限。";
});
const sqlKeywordCompletions: Completion[] = [
  "SELECT",
  "WITH",
  "FROM",
  "WHERE",
  "JOIN",
  "LEFT JOIN",
  "INNER JOIN",
  "GROUP BY",
  "ORDER BY",
  "HAVING",
  "LIMIT",
  "FETCH FIRST",
  "TOP",
  "COUNT",
  "SUM",
  "AVG",
  "MIN",
  "MAX",
  "DISTINCT",
  "CASE",
  "WHEN",
  "THEN",
  "ELSE",
  "END",
].map((label) => ({ label, type: "keyword" }));
const sqlEditorTheme = EditorView.theme({
  "&": {
    height: "100%",
    fontSize: "14px",
  },
  ".cm-scroller": {
    fontFamily: "Consolas, Monaco, monospace",
    lineHeight: "1.6",
  },
});
let queryShortcutHandler = () => {};
const queryShortcutKeymap = Prec.highest(
  keymap.of([
    {
      key: "Ctrl-Enter",
      run: () => {
        queryShortcutHandler();
        return true;
      },
    },
    {
      key: "Mod-Enter",
      run: () => {
        queryShortcutHandler();
        return true;
      },
    },
  ]),
);
const metadataCompletions = computed<Completion[]>(() => {
  const items = new Map<string, Completion>();
  for (const item of metadataTables.value) {
    items.set(`table:${item.name}`, {
      label: item.name,
      type: "class",
      detail: item.comment || "表",
      info: item.comment || undefined,
    });
  }
  for (const item of metadataColumns.value) {
    items.set(`column:${item.tableName}.${item.columnName}`, {
      label: item.columnName,
      type: "property",
      detail: `${item.tableName} ${item.columnType}`,
      info: item.comment || undefined,
    });
  }
  return [...sqlKeywordCompletions, ...items.values()];
});

const normalizeSQLName = (value: string) =>
  value
    .trim()
    .replace(/\s*\.\s*/g, ".")
    .split(".")
    .map((part) => part.replace(/^[`"\[]|[`"\]]$/g, ""))
    .join(".")
    .toLowerCase();

const resolveMetadataTableName = (tableName: string) => {
  const normalized = normalizeSQLName(tableName);
  return (
    metadataTables.value.find((item) => {
      const candidate = normalizeSQLName(item.name);
      return (
        candidate === normalized ||
        candidate.endsWith(`.${normalized}`) ||
        normalized.endsWith(`.${candidate}`)
      );
    })?.name || tableName
  );
};

const buildAliasMap = (sqlText: string) => {
  const aliases = new Map<string, string>();
  const identifier =
    String.raw`(?:"[^"]+"|` + "`[^`]+`" + String.raw`|\[[^\]]+\]|[\w$]+)`;
  const tableRef = `${identifier}(?:\\s*\\.\\s*${identifier}){0,2}`;
  const stopWords =
    "where|join|left|right|inner|outer|full|cross|on|group|order|having|limit|fetch|union|select";
  const pattern = new RegExp(
    `\\b(?:from|join)\\s+(${tableRef})(?:\\s+(?:as\\s+)?(?!(${stopWords})\\b)(${identifier}))?`,
    "gi",
  );

  for (const match of sqlText.matchAll(pattern)) {
    const rawTableName = match[1];
    if (!rawTableName) continue;
    const tableName = resolveMetadataTableName(rawTableName);
    const tableAlias =
      match[3] || rawTableName.split(".").pop() || rawTableName;
    aliases.set(normalizeSQLName(tableAlias), tableName);
  }
  return aliases;
};

const aliasColumnCompletions = (aliasName: string, sqlText: string) => {
  const tableName = buildAliasMap(sqlText).get(normalizeSQLName(aliasName));
  if (!tableName) return [];
  const normalizedTableName = normalizeSQLName(tableName);
  return metadataColumns.value
    .filter((item) => {
      const candidate = normalizeSQLName(item.tableName);
      return (
        candidate === normalizedTableName ||
        candidate.endsWith(`.${normalizedTableName}`) ||
        normalizedTableName.endsWith(`.${candidate}`)
      );
    })
    .map(
      (item) =>
        ({
          label: item.columnName,
          type: "property",
          detail: item.columnType,
          info: item.comment || undefined,
        }) satisfies Completion,
    );
};

const sqlCompletionSource = (context: CompletionContext) => {
  const word = context.matchBefore(/[\w.$"]*/);
  if (!word || (word.from === word.to && !context.explicit)) return null;
  const dotIndex = word.text.lastIndexOf(".");
  if (dotIndex > 0) {
    const aliasName = word.text.slice(0, dotIndex);
    const options = aliasColumnCompletions(
      aliasName,
      context.state.doc.toString(),
    );
    if (options.length) {
      return {
        from: word.from + dotIndex + 1,
        options,
        validFor: /^[\w$"]*$/,
      };
    }
  }
  return {
    from: word.from,
    options: metadataCompletions.value,
    validFor: /^[\w.$"]*$/,
  };
};
const sqlEditorExtensions = computed(() => [
  sql({ upperCaseKeywords: true }),
  autocompletion({ override: [sqlCompletionSource], activateOnTyping: true }),
  queryShortcutKeymap,
  EditorView.lineWrapping,
  sqlEditorTheme,
]);
const totalPages = computed(() =>
  Math.max(1, Math.ceil(rows.value.length / pageSize.value)),
);
const pagedRows = computed(() =>
  rows.value.slice(
    (currentPageNo.value - 1) * pageSize.value,
    currentPageNo.value * pageSize.value,
  ),
);
const filteredColumns = computed(() =>
  selectedTable.value
    ? metadataColumns.value.filter(
        (item) => item.tableName === selectedTable.value,
      )
    : [],
);

// 切换数据库类型或连接后，需要重新加载对应连接下的元数据。
watch(dbType, () => {
  selectedConnectionName.value = filteredConnections.value[0]?.name || "";
  resultVisible.value = false;
  resultMaximized.value = false;
  clearMetadata();
});

watch(selectedConnectionName, () => {
  resultVisible.value = false;
  resultMaximized.value = false;
  clearMetadata();
  if (selectedConnectionName.value) loadMetadata();
});

watch(pageSize, () => {
  currentPageNo.value = 1;
});

watch(metadataKeyword, () => {
  window.clearTimeout(metadataSearchTimer);
  metadataSearchTimer = window.setTimeout(() => {
    void loadMetadata();
  }, 300);
});

// 会话失效时立即清空业务页状态，避免未登录用户看到上一次的数据。
const enterUnauthenticatedState = (reason = "") => {
  currentUser.value = null;
  page.value = "query";
  connections.value = [];
  selectedConnectionName.value = "";
  adminConnections.value = [];
  users.value = [];
  columns.value = [];
  rows.value = [];
  clearMetadata();
  history.value = [];
  resultVisible.value = false;
  resultMaximized.value = false;
  favoritesVisible.value = false;
  connectionFormVisible.value = false;
  userFormVisible.value = false;
  changePwdVisible.value = false;
  if (reason) loginMessage.value = reason;
};

// 统一封装 JSON API 调用，并将所有 401 响应收敛到固定登录页。
const apiJSON = async (url: string, options: RequestInit = {}) => {
  const res = await fetch(url, {
    credentials: "include",
    cache: "no-store",
    ...options,
  });
  const data = await res.json();
  if (res.status === 401) {
    enterUnauthenticatedState(
      url === "/api/auth/me" ? "" : data.message || "登录已失效，请重新登录",
    );
    throw new Error(data.message || "未登录");
  }
  if (!res.ok || data.ok === false) throw new Error(data.message || "请求失败");
  return data;
};

const postJSON = (url: string, body: unknown) =>
  apiJSON(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

const putJSON = (url: string, body: unknown) =>
  apiJSON(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

const deleteJSON = (url: string, body: unknown) =>
  apiJSON(url, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

// 页面初始化时先确认会话，已登录则加载查询连接和本地历史。
const checkAuth = async () => {
  try {
    const data = await apiJSON("/api/auth/me");
    currentUser.value = data.user;
    await loadQueryConnections();
    loadHistory();
  } catch {
    enterUnauthenticatedState();
  } finally {
    authChecking.value = false;
  }
};

// 登录成功后按后端标记决定是否强制提示修改密码。
const login = async () => {
  loginLoading.value = true;
  loginMessage.value = "";
  try {
    const data = await postJSON("/api/login", loginForm.value);
    currentUser.value = data.user;
    if (currentUser.value?.needChangePwd === 1) changePwdVisible.value = true;
    await loadQueryConnections();
    loadHistory();
  } catch (err) {
    loginMessage.value = err instanceof Error ? err.message : "登录失败";
  } finally {
    loginLoading.value = false;
  }
};

// 退出登录只清理页面状态，本地历史仍按用户保留，重新登录后会自动恢复。
const logout = async () => {
  try {
    await postJSON("/api/logout", {});
  } finally {
    loginMessage.value = "";
    enterUnauthenticatedState();
  }
};

// 当前用户修改密码，前端先做必填和两次输入一致性校验。
const changePassword = async () => {
  changePwdMessage.value = "";
  if (!changePwdForm.value.oldPassword || !changePwdForm.value.newPassword) {
    changePwdMessage.value = "请填写完整密码";
    return;
  }
  if (changePwdForm.value.newPassword !== changePwdForm.value.confirmPassword) {
    changePwdMessage.value = "两次新密码不一致";
    return;
  }
  try {
    await postJSON("/api/user/change-password", changePwdForm.value);
    changePwdVisible.value = false;
    changePwdForm.value = {
      oldPassword: "",
      newPassword: "",
      confirmPassword: "",
    };
    if (currentUser.value) currentUser.value.needChangePwd = 0;
  } catch (err) {
    changePwdMessage.value = err instanceof Error ? err.message : "修改失败";
  }
};

// 查询连接由后端按当前用户权限过滤，普通用户只能看到已授权连接。
const loadQueryConnections = async () => {
  if (!isAuthenticated.value) return;
  const data = await apiJSON("/api/query-connections");
  connections.value = data.connections || [];
  selectedConnectionName.value = filteredConnections.value[0]?.name || "";
};

// 执行查询时只提交连接名和 SQL，SELECT/WITH 限制由后端统一兜底。
const executeQuery = async () => {
  if (!selectedConnectionName.value) {
    queryMessage.value = "请选择数据库连接";
    return;
  }
  queryLoading.value = true;
  queryMessage.value = "查询中...";
  try {
    const data = await postJSON("/api/query-data", {
      connectionName: selectedConnectionName.value,
      sql: queryText.value,
    });
    columns.value = data.columns || [];
    rows.value = data.rows || [];
    currentPageNo.value = 1;
    queryMessage.value = `${data.message || "查询成功"}，返回 ${data.rowCount || rows.value.length} 行`;
    resultVisible.value = true;
    if (rows.value.length > 0) saveHistory();
  } catch (err) {
    columns.value = [];
    rows.value = [];
    resultVisible.value = false;
    resultMaximized.value = false;
    queryMessage.value = err instanceof Error ? err.message : "查询失败";
  } finally {
    queryLoading.value = false;
  }
};

queryShortcutHandler = () => {
  if (queryLoading.value || !canQuery.value) return;
  void executeQuery();
};

// 导出 Excel 复用查询接口参数，文件内容由后端按当前查询结果生成。
const exportExcel = async () => {
  exportLoading.value = true;
  try {
    const res = await fetch("/api/query-export-excel", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({
        connectionName: selectedConnectionName.value,
        sql: queryText.value,
      }),
    });
    if (res.status === 401) {
      enterUnauthenticatedState("登录已失效，请重新登录");
      throw new Error("未登录或登录已失效");
    }
    if (!res.ok) throw new Error("导出失败");
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `query_result_${Date.now()}.xlsx`;
    a.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    queryMessage.value = err instanceof Error ? err.message : "导出失败";
  } finally {
    exportLoading.value = false;
  }
};

// 元数据浏览按当前连接和关键字加载表、字段列表。
const loadMetadata = async () => {
  if (!selectedConnectionName.value || !canQuery.value) return;
  try {
    const data = await postJSON("/api/query-metadata", {
      connectionName: selectedConnectionName.value,
      keyword: metadataKeyword.value,
    });
    metadataTables.value = data.tables || [];
    metadataColumns.value = data.columns || [];
    if (
      selectedTable.value &&
      !metadataTables.value.some((item) => item.name === selectedTable.value)
    ) {
      selectedTable.value = "";
    }
  } catch (err) {
    message.value = err instanceof Error ? err.message : "元数据加载失败";
  }
};

// 清理当前连接下已展示的表和字段，避免跨连接展示旧数据。
const clearMetadata = () => {
  metadataTables.value = [];
  metadataColumns.value = [];
  selectedTable.value = "";
};

// 双击表名或字段名时，把名称插入到 SQL 编辑器末尾。
const appendSQL = (text: string) => {
  queryText.value = queryText.value ? `${queryText.value} ${text}` : text;
};

// 一键清空 SQL 编辑器，不改变连接、元数据和已经返回的查询结果。
const clearQueryText = () => {
  queryText.value = "";
};

// 查询历史仅从当前用户的 localStorage 读取，并兼容裁剪旧版本留下的超量记录。
const loadHistory = () => {
  try {
    const storedHistory = JSON.parse(
      localStorage.getItem(historyStorageKey.value) || "[]",
    );
    history.value = Array.isArray(storedHistory)
      ? storedHistory.slice(0, MAX_HISTORY_ITEMS)
      : [];
    localStorage.setItem(
      historyStorageKey.value,
      JSON.stringify(history.value),
    );
  } catch {
    history.value = [];
  }
};

// 查询返回数据后保存历史，同一连接中的相同 SQL 只保留最新一次。
const saveHistory = () => {
  if (!queryText.value.trim() || rows.value.length === 0) return;
  const item: QueryHistoryItem = {
    id: String(Date.now()),
    connectionName: selectedConnectionName.value,
    dbType: dbType.value,
    sql: queryText.value,
    createdAt: new Date().toLocaleString(),
  };
  history.value = [
    item,
    ...history.value.filter(
      (h) =>
        h.sql !== item.sql ||
        h.connectionName !== item.connectionName ||
        h.dbType !== item.dbType,
    ),
  ].slice(0, MAX_HISTORY_ITEMS);
  localStorage.setItem(historyStorageKey.value, JSON.stringify(history.value));
};

// 从历史记录恢复数据库类型、连接和 SQL 文本。
const applyHistory = (item: QueryHistoryItem) => {
  dbType.value = item.dbType;
  selectedConnectionName.value = item.connectionName;
  queryText.value = item.sql;
};

// 删除本地历史后立即写回 localStorage。
const deleteHistory = (id: string) => {
  history.value = history.value.filter((item) => item.id !== id);
  localStorage.setItem(historyStorageKey.value, JSON.stringify(history.value));
};

// 打开收藏弹窗时，用当前 SQL 预填新增表单。
const openHistory = () => {
  drawerTab.value = "history";
  favoritesVisible.value = true;
};

const openFavorites = async () => {
  drawerTab.value = "favorites";
  favoritesVisible.value = true;
  resetFavoriteForm();
  await loadFavorites();
};

// SQL 收藏由后端持久化，支持跨浏览器复用。
const loadFavorites = async () => {
  const data = await apiJSON("/api/sql-favorites");
  favorites.value = data.favorites || [];
};

// 重置收藏表单，并默认绑定当前查询上下文。
const resetFavoriteForm = () => {
  favoriteForm.value = {
    id: 0,
    favoriteName: "",
    sqlText: queryText.value,
    dbType: dbType.value,
    connectionName: selectedConnectionName.value,
    remark: "",
    isPinned: 0,
  };
};

// 编辑收藏时直接把收藏内容回填到表单。
const editFavorite = (item: FavoriteItem) => {
  favoriteForm.value = { ...item };
};

const favoriteNameFromSQL = (sqlText: string) => {
  const firstLine = sqlText.trim().split(/\r?\n/)[0] || "常用查询";
  return firstLine.length > 36 ? `${firstLine.slice(0, 36)}...` : firstLine;
};

const addHistoryToFavorite = async (item: QueryHistoryItem) => {
  await postJSON("/api/sql-favorites", {
    id: 0,
    favoriteName: favoriteNameFromSQL(item.sql),
    sqlText: item.sql,
    dbType: item.dbType,
    connectionName: item.connectionName,
    remark: `来自查询历史 ${item.createdAt}`,
    isPinned: 0,
  });
  queryMessage.value = "已加入 SQL 收藏";
  drawerTab.value = "favorites";
  await loadFavorites();
};

// 保存收藏时按是否存在 id 自动选择新增或更新。
const saveFavorite = async () => {
  if (favoriteForm.value.id) {
    await putJSON("/api/sql-favorites", favoriteForm.value);
  } else {
    await postJSON("/api/sql-favorites", favoriteForm.value);
  }
  resetFavoriteForm();
  await loadFavorites();
};

// 使用收藏会切换连接并把 SQL 放回编辑器。
const useFavorite = (item: FavoriteItem) => {
  dbType.value = item.dbType;
  selectedConnectionName.value = item.connectionName;
  queryText.value = item.sqlText;
  favoritesVisible.value = false;
};

// 删除收藏前保留一次浏览器确认，避免误删常用 SQL。
const removeFavorite = async (item: FavoriteItem) => {
  if (!confirm(`删除收藏「${item.favoriteName}」？`)) return;
  await deleteJSON("/api/sql-favorites", { id: item.id });
  await loadFavorites();
};

// 切换到管理页时按需加载对应数据，查询页不做额外请求。
const navigate = async (target: Page) => {
  page.value = target;
  message.value = "";
  if (target === "connections") await loadAdminConnections();
  if (target === "users") await loadUsers();
};

// 管理员连接列表用于连接管理，也给用户授权表单复用。
const loadAdminConnections = async () => {
  const data = await apiJSON("/api/admin/db-connections");
  adminConnections.value = data.connections || [];
};

// 管理列表中按数据库类型展示连接目标。
const connectionTargetText = (item: AdminConnection) => {
  if (item.dbType === "oracle")
    return item.databaseName
      ? `${item.serviceName}/${item.databaseName}`
      : item.serviceName;
  if (item.dbType === "postgres")
    return item.serviceName
      ? `${item.databaseName}/${item.serviceName}`
      : item.databaseName;
  return item.databaseName;
};

// 新增连接时按 MySQL 默认端口初始化表单。
const newConnection = () => {
  connectionFormMessage.value = "";
  connectionForm.value = {
    id: 0,
    name: "",
    dbType: "mysql",
    host: "",
    port: 3306,
    username: "",
    password: "",
    databaseName: "",
    serviceName: "",
    isEnabled: 1,
    canConnect: 1,
  };
  connectionFormVisible.value = true;
};

// 不同数据库类型使用各自常见端口。
const setDefaultConnectionPort = () => {
  if (connectionForm.value.dbType === "oracle")
    connectionForm.value.port = 1521;
  else if (connectionForm.value.dbType === "postgres")
    connectionForm.value.port = 5432;
  else if (connectionForm.value.dbType === "mssql")
    connectionForm.value.port = 1433;
  else connectionForm.value.port = 3306;
};

// 切换连接类型时清理不再适用的字段，避免保存无意义的残留值。
const handleConnectionTypeChange = () => {
  connectionFormMessage.value = "";
  setDefaultConnectionPort();
  if (connectionForm.value.dbType === "oracle") {
    return;
  } else if (connectionForm.value.dbType !== "postgres") {
    connectionForm.value.serviceName = "";
  }
};

// 编辑连接时不回显密码，留空表示沿用原密码。
const editConnection = (item: AdminConnection) => {
  connectionFormMessage.value = "";
  connectionForm.value = { ...item, password: "" };
  connectionFormVisible.value = true;
};

// 保存连接前若允许连接，会先调用后端测试端口和数据库登录。
const saveConnection = async () => {
  connectionFormMessage.value = "";
  try {
    const payload = connectionForm.value;
    if (payload.canConnect === 1)
      await postJSON("/api/admin/db-connections/test", payload);
    if (payload.id) await putJSON("/api/admin/db-connections", payload);
    else await postJSON("/api/admin/db-connections", payload);
    connectionFormVisible.value = false;
    await loadAdminConnections();
    await loadQueryConnections();
  } catch (err) {
    const text = err instanceof Error ? err.message : "连接保存失败";
    connectionFormMessage.value = text;
  }
};

// 删除连接后刷新管理列表和查询工作台可用连接。
const removeConnection = async (item: AdminConnection) => {
  if (!confirm(`删除连接「${item.name}」？`)) return;
  await deleteJSON("/api/admin/db-connections", { id: item.id });
  await loadAdminConnections();
  await loadQueryConnections();
};

// 加载用户列表时同时刷新连接列表，保证授权选项是最新的。
const loadUsers = async () => {
  const data = await apiJSON("/api/admin/users");
  users.value = data.users || [];
  await loadAdminConnections();
};

// 新增用户默认给普通查询用户权限，连接授权由管理员勾选。
const newUser = () => {
  userForm.value = {
    id: 0,
    username: "",
    password: "",
    displayName: "",
    roleName: "user",
    isEnabled: 1,
    canQueryData: 1,
    allowedConnections: [],
  };
  userFormVisible.value = true;
};

// 编辑用户时复制连接授权数组，避免直接修改表格中的原对象。
const editUser = (item: UserItem) => {
  userForm.value = {
    ...item,
    password: "",
    allowedConnections: [...(item.allowedConnections || [])],
  };
  userFormVisible.value = true;
};

// 用户授权通过勾选连接名称维护。
const toggleUserConnection = (name: string) => {
  const list = userForm.value.allowedConnections;
  const idx = list.indexOf(name);
  if (idx >= 0) list.splice(idx, 1);
  else list.push(name);
};

// 保存用户时按是否存在 id 自动选择新增或更新。
const saveUser = async () => {
  if (userForm.value.id) await putJSON("/api/admin/users", userForm.value);
  else await postJSON("/api/admin/users", userForm.value);
  userFormVisible.value = false;
  await loadUsers();
};

// 删除用户前保留一次浏览器确认。
const removeUser = async (item: UserItem) => {
  if (!confirm(`删除用户「${item.username}」？`)) return;
  await deleteJSON("/api/admin/users", { id: item.id });
  await loadUsers();
};

// 查询结果支持从上边缘拖动调整高度，并记住本机浏览器的最后设置。
const RESULT_HEIGHT_KEY = "db_query_result_height";
let resultResizeStartY = 0;
let resultResizeStartHeight = 0;

const clampResultHeight = (height: number) => {
  const maxHeight = Math.max(320, window.innerHeight - 180);
  return Math.min(Math.max(height, 320), maxHeight);
};

const resizeQueryResult = (event: PointerEvent) => {
  resultHeight.value = clampResultHeight(
    resultResizeStartHeight + resultResizeStartY - event.clientY,
  );
};

const stopResultResize = () => {
  window.removeEventListener("pointermove", resizeQueryResult);
  window.removeEventListener("pointerup", stopResultResize);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  localStorage.setItem(RESULT_HEIGHT_KEY, String(resultHeight.value));
};

const startResultResize = (event: PointerEvent) => {
  if (resultMaximized.value) return;
  event.preventDefault();
  resultResizeStartY = event.clientY;
  resultResizeStartHeight = resultHeight.value;
  document.body.style.cursor = "row-resize";
  document.body.style.userSelect = "none";
  window.addEventListener("pointermove", resizeQueryResult);
  window.addEventListener("pointerup", stopResultResize);
};

const resetResultHeight = () => {
  resultHeight.value = clampResultHeight(Math.round(window.innerHeight * 0.55));
  localStorage.setItem(RESULT_HEIGHT_KEY, String(resultHeight.value));
};

const hideQueryResult = () => {
  resultVisible.value = false;
  resultMaximized.value = false;
};

// 元数据栏折叠状态保存在浏览器中，展开后继续使用已经加载的表和字段。
const METADATA_COLLAPSED_KEY = "db_query_metadata_collapsed";
const toggleMetadataPanel = () => {
  metadataCollapsed.value = !metadataCollapsed.value;
  localStorage.setItem(
    METADATA_COLLAPSED_KEY,
    metadataCollapsed.value ? "1" : "0",
  );
};

onMounted(() => {
  const storedHeight = Number(localStorage.getItem(RESULT_HEIGHT_KEY));
  resultHeight.value = clampResultHeight(
    Number.isFinite(storedHeight) && storedHeight > 0 ? storedHeight : 520,
  );
  metadataCollapsed.value =
    localStorage.getItem(METADATA_COLLAPSED_KEY) === "1";
  void checkAuth();
});
onBeforeUnmount(() => {
  window.clearTimeout(metadataSearchTimer);
  stopResultResize();
});
</script>

<template>
  <div class="app">
    <main v-if="authChecking" class="auth-page">
      <div class="auth-status" role="status">
        <span class="brand-mark"><DatabaseIcon :size="24" /></span>
        <strong>DB Platform</strong>
        <span class="auth-loader"></span>
        <small>正在验证登录状态</small>
      </div>
    </main>

    <main v-else-if="!isAuthenticated" class="auth-page">
      <form class="auth-panel" @submit.prevent="login">
        <div class="auth-brand">
          <span class="brand-mark"><DatabaseIcon :size="24" /></span>
          <div>
            <strong>DB Platform</strong>
          </div>
        </div>
        <div class="auth-heading">
          <h1>数据库查询平台</h1>
          <p>请使用已授权账号登录</p>
        </div>
        <label>
          <span>用户名</span>
          <input v-model="loginForm.username" autocomplete="username" />
        </label>
        <label>
          <span>密码</span>
          <input
            v-model="loginForm.password"
            type="password"
            autocomplete="current-password"
          />
        </label>
        <p v-if="loginMessage" class="auth-error">{{ loginMessage }}</p>
        <button class="btn primary auth-submit" :disabled="loginLoading">
          <ShieldCheckIcon :size="17" />{{
            loginLoading ? "登录中..." : "登录"
          }}
        </button>
      </form>
    </main>

    <aside
      v-if="isAuthenticated"
      class="sidebar"
      :class="{ collapsed: sidebarCollapsed }"
    >
      <div class="brand">
        <span class="brand-mark"><DatabaseIcon :size="22" /></span>
        <span class="brand-copy">
          <strong>DB Platform</strong>
        </span>
      </div>
      <nav class="nav">
        <button
          :class="{ active: page === 'query' }"
          @click="navigate('query')"
        >
          <SearchIcon :size="18" />
          <span>查询工作台</span>
        </button>
        <button
          v-if="isAdmin"
          :class="{ active: page === 'connections' }"
          @click="navigate('connections')"
        >
          <ServerCogIcon :size="18" />
          <span>数据库连接</span>
        </button>
        <button
          v-if="isAdmin"
          :class="{ active: page === 'users' }"
          @click="navigate('users')"
        >
          <UsersIcon :size="18" />
          <span>用户权限</span>
        </button>
      </nav>
      <button
        class="collapse-btn"
        :title="sidebarCollapsed ? '展开侧栏' : '折叠侧栏'"
        @click="sidebarCollapsed = !sidebarCollapsed"
      >
        <PanelLeftOpenIcon v-if="sidebarCollapsed" :size="18" />
        <PanelLeftCloseIcon v-else :size="18" />
        <span>{{ sidebarCollapsed ? "展开侧栏" : "折叠侧栏" }}</span>
      </button>
    </aside>

    <main v-if="isAuthenticated" class="main">
      <header class="topbar">
        <div class="page-heading">
          <strong>{{ pageTitle }}</strong>
          <span v-if="page === 'query'"
            >{{ dbType.toUpperCase() }} ·
            {{ selectedConnectionName || "未选择连接" }}</span
          >
          <span v-else-if="page === 'connections'"
            >{{ adminConnections.length }} 个连接</span
          >
          <span v-else>{{ users.length }} 个用户</span>
        </div>
        <div class="userbar" v-if="currentUser">
          <span class="avatar"><UserIcon :size="16" /></span>
          <span>{{ currentUser.displayName || currentUser.username }}</span>
          <button class="btn ghost" @click="changePwdVisible = true">
            <KeyRoundIcon :size="16" />改密码
          </button>
          <button class="btn ghost danger-text" @click="logout">
            <LogOutIcon :size="16" />退出
          </button>
        </div>
      </header>

      <section
        v-if="page === 'query'"
        class="query-layout"
        :class="{
          'has-results': resultVisible,
          'result-maximized': resultMaximized,
        }"
      >
        <div
          class="query-top"
          :class="{ 'metadata-collapsed': metadataCollapsed }"
        >
          <section class="panel workspace">
            <div class="toolbar">
              <select v-model="dbType">
                <option value="mysql">MySQL</option>
                <option value="oracle">Oracle</option>
                <option value="postgres">PostgreSQL</option>
                <option value="mssql">MSSQL</option>
              </select>
              <select v-model="selectedConnectionName">
                <option
                  v-for="item in filteredConnections"
                  :key="item.name"
                  :value="item.name"
                >
                  {{ item.name }}
                </option>
              </select>
              <button
                class="btn primary"
                :disabled="queryLoading || !canQuery"
                @click="executeQuery"
              >
                <PlayIcon :size="16" />{{
                  queryLoading ? "查询中..." : "执行查询"
                }}
              </button>
              <button
                class="icon-btn"
                title="清空查询框"
                :disabled="!queryText"
                @click="clearQueryText"
              >
                <EraserIcon :size="16" />
              </button>
              <button
                class="btn secondary"
                :disabled="exportLoading || queryLoading"
                @click="exportExcel"
              >
                <DownloadIcon :size="16" />导出 Excel
              </button>
              <button class="btn secondary" @click="openHistory">
                <HistoryIcon :size="16" />查询历史
              </button>
              <button class="btn secondary" @click="openFavorites">
                <BookmarkIcon :size="16" />SQL 收藏
              </button>
            </div>
            <div v-if="selectedConnection" class="hint">
              {{ selectedConnection.host }}:{{ selectedConnection.port }}
              <span v-if="!canQuery">该连接标记为不可查询</span>
            </div>
            <Codemirror
              v-model="queryText"
              class="editor sql-editor"
              placeholder="仅支持 SELECT / WITH 查询，最多返回 500 行"
              :extensions="sqlEditorExtensions"
              :indent-with-tab="true"
              :tab-size="2"
            />
            <div v-if="!resultVisible" class="result-msg">
              {{ queryMessage }}
            </div>
          </section>

          <aside class="panel side" :class="{ collapsed: metadataCollapsed }">
            <div class="panel-title metadata-title">
              <span v-if="!metadataCollapsed" class="metadata-title-copy">
                <Table2Icon :size="17" />
                <span>元数据</span>
              </span>
              <button
                class="icon-btn metadata-toggle"
                :title="metadataCollapsed ? '展开元数据栏' : '折叠元数据栏'"
                @click="toggleMetadataPanel"
              >
                <PanelRightOpenIcon v-if="metadataCollapsed" :size="17" />
                <PanelRightCloseIcon v-else :size="17" />
              </button>
            </div>
            <template v-if="!metadataCollapsed">
              <div class="row">
                <input
                  v-model="metadataKeyword"
                  placeholder="表名/字段名"
                  @keyup.enter="loadMetadata"
                />
                <button
                  class="icon-btn"
                  title="刷新元数据"
                  @click="loadMetadata"
                >
                  <RefreshCwIcon :size="16" />
                </button>
              </div>
              <div class="scroll-list">
                <button
                  v-for="item in metadataTables"
                  :key="item.name"
                  class="list-item"
                  @click="selectedTable = item.name"
                  @dblclick="appendSQL(item.name)"
                >
                  <strong>{{ item.name }}</strong>
                  <small>{{ item.comment }}</small>
                </button>
              </div>
              <div class="panel-title small">
                <Columns3Icon :size="17" />
                <span>字段</span>
                <small class="table-context" v-if="selectedTable">{{
                  selectedTable
                }}</small>
              </div>
              <div class="scroll-list compact">
                <div v-if="!selectedTable" class="metadata-empty">
                  请选择表查看字段
                </div>
                <button
                  v-for="item in filteredColumns"
                  :key="`${item.tableName}.${item.columnName}`"
                  class="list-item"
                  @dblclick="appendSQL(item.columnName)"
                >
                  <span class="list-row">
                    <strong>{{ item.columnName }}</strong>
                    <small>{{ item.columnType }}</small>
                  </span>
                  <small class="comment" v-if="item.comment">{{
                    item.comment
                  }}</small>
                  <small class="comment empty" v-else>暂无字段注释</small>
                </button>
              </div>
            </template>
          </aside>
        </div>

        <section
          v-if="resultVisible"
          class="panel query-result-panel"
          :style="{ '--result-height': `${resultHeight}px` }"
        >
          <div
            class="result-resize-handle"
            title="拖动调整结果区域高度，双击恢复默认高度"
            @pointerdown="startResultResize"
            @dblclick="resetResultHeight"
          ></div>
          <div class="result-head">
            <div>
              <strong>查询结果</strong>
              <span>{{ queryMessage }}</span>
            </div>
            <div class="result-actions">
              <button
                class="icon-btn"
                :title="resultMaximized ? '退出最大化' : '最大化查询结果'"
                @click="resultMaximized = !resultMaximized"
              >
                <Minimize2Icon v-if="resultMaximized" :size="16" />
                <Maximize2Icon v-else :size="16" />
              </button>
              <button
                class="icon-btn"
                title="隐藏查询结果"
                @click="hideQueryResult"
              >
                <XIcon :size="17" />
              </button>
            </div>
          </div>
          <div class="table-wrap result-table-wrap">
            <table v-if="columns.length">
              <thead>
                <tr>
                  <th v-for="col in columns" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, idx) in pagedRows" :key="idx">
                  <td v-for="col in columns" :key="col">{{ row[col] }}</td>
                </tr>
              </tbody>
            </table>
            <div v-else class="empty-result">查询成功，未返回可展示的字段</div>
          </div>
          <div class="pager">
            <label class="page-size">
              <span>每页</span>
              <select v-model.number="pageSize">
                <option
                  v-for="size in pageSizeOptions"
                  :key="size"
                  :value="size"
                >
                  {{ size }} 行
                </option>
              </select>
            </label>
            <button :disabled="currentPageNo <= 1" @click="currentPageNo--">
              上一页
            </button>
            <span>{{ currentPageNo }} / {{ totalPages }}</span>
            <button
              :disabled="currentPageNo >= totalPages"
              @click="currentPageNo++"
            >
              下一页
            </button>
          </div>
        </section>
      </section>

      <section v-else-if="page === 'connections'" class="panel">
        <div class="section-head">
          <h2>数据库连接</h2>
          <button class="btn primary" @click="newConnection">
            <PlusIcon :size="16" />新增连接
          </button>
        </div>
        <table>
          <thead>
            <tr>
              <th>名称</th>
              <th>类型</th>
              <th>主机</th>
              <th>端口</th>
              <th>账号</th>
              <th>库/服务</th>
              <th>启用</th>
              <th>可连接</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in adminConnections" :key="item.id">
              <td>{{ item.name }}</td>
              <td>{{ item.dbType }}</td>
              <td>{{ item.host }}</td>
              <td>{{ item.port }}</td>
              <td>{{ item.username }}</td>
              <td>{{ connectionTargetText(item) }}</td>
              <td>{{ item.isEnabled ? "是" : "否" }}</td>
              <td>{{ item.canConnect ? "是" : "否" }}</td>
              <td class="table-actions">
                <button
                  class="icon-btn"
                  title="编辑连接"
                  @click="editConnection(item)"
                >
                  <PencilIcon :size="15" />
                </button>
                <button
                  class="icon-btn danger"
                  title="删除连接"
                  @click="removeConnection(item)"
                >
                  <Trash2Icon :size="15" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>

      <section v-else class="panel">
        <div class="section-head">
          <h2>用户权限</h2>
          <button class="btn primary" @click="newUser">
            <PlusIcon :size="16" />新增用户
          </button>
        </div>
        <table>
          <thead>
            <tr>
              <th>用户名</th>
              <th>显示名</th>
              <th>角色</th>
              <th>启用</th>
              <th>查询权限</th>
              <th>连接权限</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in users" :key="item.id">
              <td>{{ item.username }}</td>
              <td>{{ item.displayName }}</td>
              <td>{{ item.roleName }}</td>
              <td>{{ item.isEnabled ? "是" : "否" }}</td>
              <td>{{ item.canQueryData ? "允许" : "禁止" }}</td>
              <td>{{ (item.allowedConnections || []).join(", ") || "-" }}</td>
              <td class="table-actions">
                <button
                  class="icon-btn"
                  title="编辑用户"
                  @click="editUser(item)"
                >
                  <PencilIcon :size="15" />
                </button>
                <button
                  class="icon-btn danger"
                  title="删除用户"
                  @click="removeUser(item)"
                >
                  <Trash2Icon :size="15" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </main>

    <div v-if="changePwdVisible" class="modal">
      <form class="dialog" @submit.prevent="changePassword">
        <h2><KeyRoundIcon :size="21" />修改密码</h2>
        <input
          v-model="changePwdForm.oldPassword"
          type="password"
          placeholder="原密码"
        />
        <input
          v-model="changePwdForm.newPassword"
          type="password"
          placeholder="新密码"
        />
        <input
          v-model="changePwdForm.confirmPassword"
          type="password"
          placeholder="确认新密码"
        />
        <p>{{ changePwdMessage }}</p>
        <div class="actions">
          <button class="btn primary"><SaveIcon :size="16" />保存</button>
          <button
            class="btn secondary"
            type="button"
            v-if="currentUser?.needChangePwd !== 1"
            @click="changePwdVisible = false"
          >
            <XIcon :size="16" />取消
          </button>
        </div>
      </form>
    </div>

    <div
      v-if="favoritesVisible"
      class="drawer-mask"
      @click.self="favoritesVisible = false"
    >
      <aside class="drawer-panel">
        <div class="drawer-head">
          <div class="drawer-tabs">
            <button
              :class="{ active: drawerTab === 'history' }"
              @click="drawerTab = 'history'"
            >
              <HistoryIcon :size="16" />查询历史
            </button>
            <button
              :class="{ active: drawerTab === 'favorites' }"
              @click="
                drawerTab = 'favorites';
                loadFavorites();
              "
            >
              <BookmarkIcon :size="16" />SQL 收藏
            </button>
          </div>
          <button
            class="icon-btn"
            title="关闭"
            @click="favoritesVisible = false"
          >
            <XIcon :size="16" />
          </button>
        </div>

        <div v-if="drawerTab === 'history'" class="drawer-body">
          <div class="scroll-list">
            <div v-for="item in history" :key="item.id" class="history-item">
              <button
                @click="
                  applyHistory(item);
                  favoritesVisible = false;
                "
              >
                {{ item.sql }}
              </button>
              <small>{{ item.connectionName }} {{ item.createdAt }}</small>
              <div class="history-actions">
                <button
                  class="text-action"
                  title="加入 SQL 收藏"
                  @click="addHistoryToFavorite(item)"
                >
                  <StarIcon :size="14" />收藏
                </button>
                <button
                  class="text-action danger-text"
                  @click="deleteHistory(item.id)"
                >
                  <Trash2Icon :size="14" />删除
                </button>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="drawer-body">
          <div class="favorite-form">
            <input v-model="favoriteForm.favoriteName" placeholder="收藏名称" />
            <select v-model="favoriteForm.connectionName">
              <option value="">不绑定连接</option>
              <option
                v-for="item in connections"
                :key="item.name"
                :value="item.name"
              >
                {{ item.name }}
              </option>
            </select>
            <textarea
              v-model="favoriteForm.sqlText"
              class="small-editor"
              placeholder="SQL"
            ></textarea>
            <input v-model="favoriteForm.remark" placeholder="备注" />
            <button class="btn primary" @click="saveFavorite">
              <SaveIcon :size="16" />保存收藏
            </button>
          </div>
          <div class="scroll-list favorites">
            <div v-for="item in favorites" :key="item.id" class="favorite-row">
              <strong>{{ item.favoriteName }}</strong>
              <span>{{ item.connectionName || "-" }}</span>
              <button
                class="icon-btn"
                title="使用收藏"
                @click="useFavorite(item)"
              >
                <PlayIcon :size="15" />
              </button>
              <button
                class="icon-btn"
                title="编辑收藏"
                @click="editFavorite(item)"
              >
                <PencilIcon :size="15" />
              </button>
              <button
                class="icon-btn danger"
                title="删除收藏"
                @click="removeFavorite(item)"
              >
                <Trash2Icon :size="15" />
              </button>
            </div>
          </div>
        </div>
      </aside>
    </div>

    <div v-if="connectionFormVisible" class="modal">
      <form class="dialog wide" @submit.prevent="saveConnection">
        <h2>
          <ServerCogIcon :size="21" />{{
            connectionForm.id ? "编辑连接" : "新增连接"
          }}
        </h2>
        <div class="grid two">
          <input v-model="connectionForm.name" placeholder="连接名称" />
          <select
            v-model="connectionForm.dbType"
            @change="handleConnectionTypeChange"
          >
            <option value="mysql">mysql</option>
            <option value="oracle">oracle</option>
            <option value="postgres">postgres</option>
            <option value="mssql">mssql</option>
          </select>
          <input v-model="connectionForm.host" placeholder="主机" />
          <input
            v-model.number="connectionForm.port"
            type="number"
            placeholder="端口"
          />
          <input v-model="connectionForm.username" placeholder="账号" />
          <input
            v-model="connectionForm.password"
            type="password"
            placeholder="密码，编辑时留空表示不修改"
          />
          <input
            v-if="connectionForm.dbType !== 'oracle'"
            v-model="connectionForm.databaseName"
            placeholder="数据库名"
          />
          <input
            v-if="connectionForm.dbType === 'postgres'"
            v-model="connectionForm.serviceName"
            placeholder="Schema 名，留空使用默认 schema"
          />
          <input
            v-if="connectionForm.dbType === 'oracle'"
            v-model="connectionForm.serviceName"
            placeholder="Oracle 服务名"
          />
          <input
            v-if="connectionForm.dbType === 'oracle'"
            v-model="connectionForm.databaseName"
            placeholder="Oracle Schema 名，留空使用当前用户"
          />
          <select v-model.number="connectionForm.isEnabled">
            <option :value="1">启用</option>
            <option :value="0">禁用</option>
          </select>
          <select v-model.number="connectionForm.canConnect">
            <option :value="1">可连接</option>
            <option :value="0">不可连接</option>
          </select>
        </div>
        <p class="form-hint">{{ connectionPermissionHint }}</p>
        <p v-if="connectionFormMessage" class="form-error">
          {{ connectionFormMessage }}
        </p>
        <div class="actions">
          <button class="btn primary"><SaveIcon :size="16" />保存</button>
          <button
            class="btn secondary"
            type="button"
            @click="connectionFormVisible = false"
          >
            <XIcon :size="16" />取消
          </button>
        </div>
      </form>
    </div>

    <div v-if="userFormVisible" class="modal">
      <form class="dialog wide" @submit.prevent="saveUser">
        <h2>
          <UsersIcon :size="21" />{{ userForm.id ? "编辑用户" : "新增用户" }}
        </h2>
        <div class="grid two">
          <input v-model="userForm.username" placeholder="用户名" />
          <input
            v-model="userForm.password"
            type="password"
            placeholder="密码，编辑时留空表示不修改"
          />
          <input v-model="userForm.displayName" placeholder="显示名" />
          <select v-model="userForm.roleName">
            <option value="user">user</option>
            <option value="admin">admin</option>
          </select>
          <select v-model.number="userForm.isEnabled">
            <option :value="1">启用</option>
            <option :value="0">禁用</option>
          </select>
          <select v-model.number="userForm.canQueryData">
            <option :value="1">允许查询</option>
            <option :value="0">禁止查询</option>
          </select>
        </div>
        <div v-if="userForm.roleName === 'user'" class="check-list">
          <label v-for="item in adminConnections" :key="item.name">
            <input
              type="checkbox"
              :checked="userForm.allowedConnections.includes(item.name)"
              @change="toggleUserConnection(item.name)"
            />
            {{ item.name }}
          </label>
        </div>
        <div class="actions">
          <button class="btn primary"><SaveIcon :size="16" />保存</button>
          <button
            class="btn secondary"
            type="button"
            @click="userFormVisible = false"
          >
            <XIcon :size="16" />取消
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  background: #eef2f7;
  color: #172033;
}

.auth-page {
  width: 100%;
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: #eef2f7;
}

.auth-panel,
.auth-status {
  width: min(400px, 100%);
  display: grid;
  gap: 18px;
  padding: 28px;
  background: #fff;
  border: 1px solid #dbe3ee;
  border-top: 4px solid #0f766e;
  border-radius: 8px;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.12);
}

.auth-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e6edf5;
}

.auth-brand strong,
.auth-brand small {
  display: block;
}

.auth-brand small {
  margin-top: 3px;
  color: #64748b;
}

.auth-heading h1 {
  margin: 0;
  font-size: 22px;
  letter-spacing: 0;
}

.auth-heading p {
  margin: 6px 0 0;
  color: #64748b;
}

.auth-panel label {
  display: grid;
  gap: 7px;
  color: #334155;
  font-weight: 600;
}

.auth-panel input {
  height: 42px;
  font-weight: 400;
}

.auth-submit {
  min-height: 42px;
}

.auth-error {
  margin: 0;
  padding: 9px 10px;
  color: #b42318;
  background: #fff1f2;
  border: 1px solid #fecdd3;
  border-radius: 6px;
}

.auth-status {
  width: 260px;
  justify-items: center;
  color: #334155;
}

.auth-loader {
  width: 26px;
  height: 26px;
  border: 3px solid #cbd5e1;
  border-top-color: #0f766e;
  border-radius: 50%;
  animation: auth-spin 0.8s linear infinite;
}

@keyframes auth-spin {
  to {
    transform: rotate(360deg);
  }
}

.sidebar {
  width: 236px;
  flex: 0 0 236px;
  position: relative;
  background: #101827;
  color: #fff;
  padding: 18px 12px;
  border-right: 1px solid #243047;
  transition:
    width 0.18s ease,
    flex-basis 0.18s ease;
}

.sidebar.collapsed {
  width: 76px;
  flex-basis: 76px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 8px 22px;
}

.sidebar.collapsed .brand {
  justify-content: center;
  padding-left: 0;
  padding-right: 0;
}

.brand-mark {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 8px;
  color: #67e8f9;
  background: linear-gradient(145deg, #1d4ed8, #0f766e);
  box-shadow: 0 10px 26px rgba(15, 118, 110, 0.28);
}

.brand strong,
.brand small {
  display: block;
}

.brand strong {
  font-size: 16px;
  letter-spacing: 0;
}

.sidebar.collapsed .brand-copy,
.sidebar.collapsed .nav span,
.sidebar.collapsed .collapse-btn span {
  display: none;
}

.brand small {
  margin-top: 3px;
  color: #93a4bc;
  font-size: 12px;
}

.nav {
  display: grid;
  gap: 6px;
}

.nav button {
  width: 100%;
  height: 42px;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  border-radius: 6px;
  padding: 0 12px;
  color: #b8c4d8;
  background: transparent;
  cursor: pointer;
}

.sidebar.collapsed .nav button {
  justify-content: center;
  padding: 0;
}

.nav button.active,
.nav button:hover {
  color: #fff;
  background: #1f2d44;
}

.nav button.active {
  box-shadow: inset 3px 0 0 #22c55e;
}

.collapse-btn {
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: 16px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #b8c4d8;
  background: #172236;
  border: 1px solid #243047;
  border-radius: 6px;
}

.collapse-btn:hover {
  color: #fff;
  background: #1f2d44;
}

.main {
  flex: 1;
  min-width: 0;
  height: 100vh;
  overflow: hidden;
}

.topbar {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 22px;
  background: rgba(255, 255, 255, 0.92);
  border-bottom: 1px solid #dbe3ee;
  backdrop-filter: blur(10px);
}

.page-heading {
  display: grid;
  gap: 3px;
}

.page-heading strong {
  font-size: 18px;
}

.page-heading span,
small {
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.userbar,
.row,
.toolbar,
.actions,
.section-head,
.table-actions,
.panel-title,
.dialog h2 {
  display: flex;
  align-items: center;
  gap: 8px;
}

.avatar {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: #0f766e;
  background: #ccfbf1;
}

.btn,
.icon-btn,
button {
  border: 0;
  border-radius: 6px;
  cursor: pointer;
  font: inherit;
}

.btn {
  min-height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 12px;
}

.btn.primary {
  color: #fff;
  background: #0f766e;
}

.btn.secondary {
  color: #172033;
  background: #e6edf5;
}

.btn.ghost {
  color: #334155;
  background: transparent;
}

.icon-btn {
  width: 34px;
  height: 34px;
  display: inline-grid;
  place-items: center;
  color: #334155;
  background: #e6edf5;
}

.icon-btn:hover,
.btn.secondary:hover,
.btn.ghost:hover {
  background: #d8e3ef;
}

.btn.primary:hover {
  background: #0d655f;
}

.icon-btn.danger,
.danger-text {
  color: #b42318;
}

button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

input,
select,
textarea {
  border: 1px solid #cad5e3;
  border-radius: 6px;
  padding: 8px 10px;
  font: inherit;
  background: #fff;
}

.query-layout {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  height: calc(100vh - 64px);
  min-width: 0;
  overflow: auto;
}

.query-top {
  min-width: 0;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 310px;
  gap: 14px;
  flex: 1 1 auto;
}

.query-top.metadata-collapsed {
  grid-template-columns: minmax(0, 1fr) 52px;
  gap: 8px;
}

.query-layout.has-results .query-top {
  flex: 0 0 clamp(280px, 34vh, 360px);
}

.query-layout.result-maximized {
  overflow: hidden;
}

.query-layout.result-maximized .query-top {
  display: none;
}

.panel {
  background: #fff;
  border: 1px solid #dbe3ee;
  border-radius: 8px;
  padding: 14px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.04);
}

.side {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;
}

.side.collapsed {
  align-items: center;
  padding: 8px 6px;
}

.metadata-title {
  width: 100%;
  justify-content: space-between;
}

.metadata-title-copy {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.side.collapsed .metadata-title {
  justify-content: center;
}

.metadata-toggle {
  flex: 0 0 34px;
}

.workspace {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-title {
  font-weight: 700;
  color: #172033;
}

.panel-title.small {
  margin-top: 8px;
}

.table-context {
  min-width: 0;
  color: #64748b;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.toolbar {
  flex-wrap: wrap;
  margin-bottom: 10px;
}

.toolbar select {
  max-width: 210px;
}

.hint {
  display: inline-flex;
  gap: 10px;
  margin-bottom: 10px;
  padding: 5px 9px;
  border-radius: 999px;
  color: #475569;
  background: #f1f5f9;
}

.hint span {
  color: #b42318;
}

.editor {
  width: 100%;
  min-height: 180px;
  font-family: Consolas, Monaco, monospace;
  line-height: 1.6;
  resize: vertical;
}

.sql-editor {
  height: 100%;
  min-height: 180px;
  flex: 1;
  overflow: hidden;
  border: 1px solid #cad5e3;
  border-radius: 6px;
  background: #fff;
}

.sql-editor :deep(.cm-editor) {
  height: 100%;
  outline: none;
}

.sql-editor :deep(.cm-focused) {
  outline: none;
}

.sql-editor :deep(.cm-scroller) {
  overflow: auto;
}

.sql-editor :deep(.cm-gutters) {
  background: #f8fafc;
  border-right: 1px solid #e6edf5;
}

.sql-editor :deep(.cm-tooltip-autocomplete) {
  border: 1px solid #dbe3ee;
  border-radius: 6px;
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.16);
}

.small-editor {
  width: 100%;
  height: 110px;
  font-family: Consolas, Monaco, monospace;
}

.result-msg {
  flex: 0 0 auto;
  margin: 10px 0;
  color: #334155;
}

.query-result-panel {
  position: relative;
  height: var(--result-height);
  min-height: 320px;
  flex: 0 0 var(--result-height);
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0;
  overflow: hidden;
}

.query-layout.result-maximized .query-result-panel {
  height: 100%;
  min-height: 0;
  flex: 1 1 auto;
}

.result-resize-handle {
  position: absolute;
  z-index: 3;
  top: 0;
  left: 0;
  right: 0;
  height: 9px;
  cursor: row-resize;
}

.result-resize-handle::after {
  content: "";
  position: absolute;
  top: 3px;
  left: 50%;
  width: 54px;
  height: 3px;
  border-radius: 2px;
  background: #cbd5e1;
  transform: translateX(-50%);
}

.result-resize-handle:hover::after {
  background: #0f766e;
}

.result-head {
  min-height: 54px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 14px 8px;
  border-bottom: 1px solid #e6edf5;
}

.result-head > div:first-child {
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.result-head strong {
  flex: 0 0 auto;
  color: #172033;
}

.result-head span {
  min-width: 0;
  color: #475569;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-actions {
  flex: 0 0 auto;
  display: flex;
  gap: 6px;
}

.query-layout.result-maximized .result-resize-handle {
  display: none;
}

.form-error {
  margin: 0;
  padding: 9px 10px;
  color: #b42318;
  background: #fff1f2;
  border: 1px solid #fecdd3;
  border-radius: 6px;
}

.form-hint {
  margin: 0;
  padding: 9px 10px;
  color: #475569;
  background: #f8fafc;
  border: 1px solid #dbe3ee;
  border-radius: 6px;
  line-height: 1.6;
}

.table-wrap {
  min-width: 0;
  max-width: 100%;
  flex: 1;
  min-height: 0;
  overflow: auto;
  border: 1px solid #dbe3ee;
  border-radius: 6px;
}

.result-table-wrap {
  margin: 0 14px;
  border-top: 0;
}

.empty-result {
  min-height: 180px;
  display: grid;
  place-items: center;
  color: #94a3b8;
}

table {
  width: max-content;
  min-width: 100%;
  border-collapse: collapse;
  background: #fff;
}

th,
td {
  border-bottom: 1px solid #e6edf5;
  padding: 7px 10px;
  text-align: left;
  vertical-align: top;
  line-height: 1.35;
}

th {
  position: sticky;
  top: 0;
  z-index: 1;
  color: #475569;
  background: #f8fafc;
}

.scroll-list {
  display: flex;
  flex-direction: column;
  gap: 7px;
  overflow: auto;
}

.scroll-list.compact {
  max-height: 260px;
}

.list-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 9px 10px;
  color: #172033;
  text-align: left;
  background: #f8fafc;
  border: 1px solid #e6edf5;
  border-radius: 6px;
}

.list-item:hover {
  background: #ecfdf5;
  border-color: #99f6e4;
}

.list-row {
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.list-row strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-row small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-item .comment {
  color: #475569;
  white-space: normal;
}

.list-item .comment.empty {
  color: #94a3b8;
}

.metadata-empty {
  padding: 12px 10px;
  color: #94a3b8;
  background: #f8fafc;
  border: 1px dashed #dbe3ee;
  border-radius: 6px;
}

.history-item {
  display: grid;
  gap: 5px;
  padding: 9px 0;
  border-bottom: 1px solid #e6edf5;
}

.history-item > button:first-child {
  max-height: 54px;
  overflow: hidden;
  padding: 0;
  color: #172033;
  text-align: left;
  background: transparent;
}

.history-actions {
  display: flex;
  gap: 12px;
}

.text-action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  width: fit-content;
  padding: 0;
  background: transparent;
}

.pager {
  flex: 0 0 auto;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  min-height: 52px;
  padding: 8px 14px;
}

.page-size {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #475569;
}

.page-size select {
  width: 96px;
}

.section-head {
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-head h2,
.dialog h2 {
  margin: 0;
  font-size: 18px;
}

.modal {
  position: fixed;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  background: rgba(15, 23, 42, 0.48);
}

.drawer-mask {
  position: fixed;
  inset: 0;
  z-index: 10;
  background: rgba(15, 23, 42, 0.28);
}

.drawer-panel {
  width: 440px;
  max-width: calc(100vw - 24px);
  height: 100vh;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  background: #fff;
  border-right: 1px solid #dbe3ee;
  box-shadow: 18px 0 42px rgba(15, 23, 42, 0.2);
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.drawer-tabs {
  display: flex;
  gap: 6px;
  padding: 4px;
  border-radius: 8px;
  background: #eef2f7;
}

.drawer-tabs button {
  height: 34px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 11px;
  color: #475569;
  background: transparent;
  border-radius: 6px;
}

.drawer-tabs button.active {
  color: #0f766e;
  background: #fff;
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.08);
}

.drawer-body {
  min-height: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 12px;
  overflow: hidden;
}

.favorite-form {
  display: grid;
  gap: 10px;
  padding-bottom: 12px;
  border-bottom: 1px solid #e6edf5;
}

.dialog {
  width: 380px;
  max-width: calc(100vw - 28px);
  display: grid;
  gap: 12px;
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.24);
}

.dialog.wide {
  width: 760px;
}

.grid.two {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.check-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding: 10px;
  border: 1px solid #e6edf5;
  border-radius: 6px;
  background: #f8fafc;
}

.favorite-row {
  display: grid;
  grid-template-columns: 1fr 140px 34px 34px 34px;
  gap: 8px;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid #e6edf5;
}

.favorites {
  max-height: 260px;
}

@media (max-width: 1100px) {
  .app {
    display: block;
  }

  .sidebar {
    width: 100%;
    min-height: auto;
  }

  .nav {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .query-top {
    grid-template-columns: 1fr;
  }

  .query-top.metadata-collapsed {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .query-layout.has-results .query-top {
    flex-basis: min(56vh, 520px);
    overflow: auto;
  }

  .side {
    height: auto;
    min-height: 280px;
    max-height: 360px;
  }

  .side.collapsed {
    min-height: 52px;
    max-height: 52px;
  }
}

@media (max-width: 720px) {
  .topbar,
  .userbar,
  .toolbar,
  .section-head {
    align-items: stretch;
    flex-direction: column;
  }

  .topbar {
    height: auto;
    padding: 12px 16px;
  }

  .query-layout {
    height: calc(100vh - 92px);
    padding: 10px;
  }

  .query-layout.has-results .query-top {
    flex-basis: 440px;
  }

  .query-result-panel {
    min-height: 360px;
  }

  .result-head,
  .result-head > div:first-child {
    align-items: flex-start;
  }

  .result-head > div:first-child {
    flex-direction: column;
    gap: 3px;
  }

  .grid.two,
  .check-list {
    grid-template-columns: 1fr;
  }

  .favorite-row {
    grid-template-columns: 1fr 34px 34px 34px;
  }

  .favorite-row span {
    display: none;
  }
}
</style>
