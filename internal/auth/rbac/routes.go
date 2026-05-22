package rbac

// RouteEntry maps an HTTP route to its RBAC resource and action.
// REQ-AUTH2-005: Table-driven route-resource mapping.
type RouteEntry struct {
	Method   string
	Path     string
	Resource string
	Action   string
}

// DefaultRoutes returns the V1 route-resource mapping table.
// 11 entries covering all protected endpoints.
var DefaultRoutes = []RouteEntry{
	{Method: "POST", Path: "/query", Resource: "query:basic", Action: "read"},
	{Method: "POST", Path: "/deep", Resource: "query:deep", Action: "read"},
	{Method: "GET", Path: "/admin/audit", Resource: "audit_log", Action: "read"},
	{Method: "POST", Path: "/admin/rbac/reload", Resource: "rbac_policy", Action: "write"},
	{Method: "POST", Path: "/admin/members", Resource: "member", Action: "write"},
	{Method: "GET", Path: "/admin/members", Resource: "member", Action: "read"},
	{Method: "DELETE", Path: "/admin/members", Resource: "member", Action: "write"},
	{Method: "POST", Path: "/admin/api-keys", Resource: "api_key", Action: "write"},
	{Method: "GET", Path: "/admin/adapter-config", Resource: "adapter_config", Action: "read"},
	{Method: "POST", Path: "/admin/adapter-config", Resource: "adapter_config", Action: "write"},
	{Method: "GET", Path: "/admin/stats", Resource: "audit_log", Action: "read"},
}
