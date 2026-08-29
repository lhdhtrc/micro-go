# access

`access` 是 go-micro 中独立的数据访问授权基础包。

它只定义资源动作、结构化行范围、字段动作和进程内决策存取，不依赖 LHDHT
permission proto、Casbin、GORM 或任何数据库。业务服务负责把 authz RPC 适配为
`DataAccessAuthorizer`，再在 biz 层显式申请并校验决策；`gormx/access` 负责之后的
静态资源绑定和参数化查询执行。

完整决策不能写入 JWT、AuthzSign 或普通 metadata。`WithDataAccessDecision` 会复制
决策切片，`DataAccessDecisionFromContext` 也返回副本，避免业务代码意外修改当前请求
的授权事实。
