# Public ID 更新说明

这次在 `main` 上已经引入了 `public_id` 机制，用来逐步替代对外直接暴露的自增 `id`。

## 这次已经做了什么

目前以下表已经加入 `public_id`：

- `users`
- `rooms`
- `room_reservations`
- `join_requests`
- `notifications`

实现方式如下：

- 每张表新增 `public_id uuid`
- 新记录创建时自动生成 UUID
- 服务启动时会给历史数据回填 `public_id`
- 对外部分响应体已经开始返回 `public_id`

当前设计原则是：

- 数据库内部主键、外键、关联关系继续使用自增 `id`
- 对外 API、前端资源标识、后续分享链接逐步切换到 `public_id`

## 当前阶段的状态

这次改动属于第一阶段：

- 数据层已经支持 `public_id`
- 历史数据会自动回填
- 部分返回体已经暴露 `public_id`

但当前外部路由还没有全部切换，很多接口路径参数仍然是整数 `id`，例如：

- `/rooms/:roomId`
- `/users/:id`

后续各组需要逐步完成第二阶段改造。

## 后续各组 API 应怎么写

统一规则：

1. 内部 service / repository / 数据库关联继续使用内部 `id`
2. 对外 HTTP API 路径参数一律逐步改用 `public_id`
3. 对外响应体如果返回主资源，应返回 `public_id`
4. 新接口从一开始就不要再设计成暴露自增 `id`

也就是说，后续推荐写法应该是：

- `GET /rooms/{roomPublicId}`
- `POST /rooms/{roomPublicId}/reservation/preview`
- `POST /rooms/{roomPublicId}/reservation/submit`
- `GET /users/{userPublicId}`

而不是继续新增：

- `/rooms/{roomId}`
- `/users/{id}`

## 各层的具体约定

### Handler 层

Handler 收到外部路径参数后，应：

1. 先按 `public_id` 查资源
2. 拿到内部记录后，再继续使用内部 `id` 调后续 service

也就是说，handler 是“外部 public_id”和“内部 id”之间的转换层。

### Repository 层

每个主资源 repository 建议同时保留两类方法：

- `GetByID(...)`
- `GetByPublicID(...)`

其中：

- `GetByID` 给内部逻辑、关联处理、已有 service 使用
- `GetByPublicID` 给 handler 层入口查询使用

### Response 层

所有对外主资源响应，建议至少包含：

- `id`：可短期保留，便于兼容
- `public_id`：作为正式对外标识

中长期目标是：

- 前端和外部调用方都只依赖 `public_id`

## 哪些表必须跟进

后续如果某一组新增或维护的是“对外主资源”，就应该同步支持 `public_id`。

典型包括：

- 用户相关主资源
- 房间相关主资源
- 预约相关主资源
- 申请、通知这类会被前端单条操作的资源

不需要跟进的通常是：

- 纯关联表
- 纯日志表
- 纯内部配置表

例如：

- `room_members`
- `reservation_attempt_logs`

这类表一般不直接作为外部资源入口，不需要单独暴露 `public_id`。

## 推荐迁移顺序

后续各组改 API 时，建议按下面顺序推进：

1. 先让返回体补齐 `public_id`
2. 再给 handler / repository 补 `GetByPublicID`
3. 再把外部路径参数从 `id` 切到 `public_id`
4. 最后逐步减少前端对整数 `id` 的依赖

## 一句话结论

从现在开始，`public_id` 应该被视为外部 API 的正式资源标识；自增 `id` 只保留给数据库内部和服务内部使用。后续各组新增或修改外部接口时，请默认按这个规则实现。
