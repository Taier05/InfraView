# Task 3 报告：共享分页控件与七页 URL

## 范围

- 仅修改共享 `ListPage` 与主机、硬盘、MySQL、Redis、Elasticsearch、RabbitMQ、Java 七个列表页及对应测试，共 16 个功能文件。
- 未修改规格、计划、progress、Service、HTTP API 或任何只读边界；未启动服务、浏览器或端口，未访问上游或私密环境。

## TDD 证据

- RED：一次性 Node 22 容器运行 8 个定向测试文件，得到 9 个预期失败；原因仅为 500 选项不存在或页面白名单尚未接受 500。
- GREEN：最小实现为七页统一 `pageSizes = [20, 50, 100, 500] as const`，共享控件将值 500 严格显示为“全部（最多500条）”。
- 测试覆盖七页切换到 500 时的 URL `page_size=500&page=1` 与最后 GET 参数；Elasticsearch、RabbitMQ、Java 的既有 runtime validator 接受 500，Host、Disk、MySQL、Redis 沿用既有 typed response path。各页均覆盖非法 URL `499` 与 `501` 规范为 `page_size=20&page=1`。
- 搜索、筛选和排序变更的页码重置测试保留；Review fix round1 另为七页补齐从合法 `page=3&page_size=500` 恢复并在状态变更后保留 500 的多页契约。

## Review fix round1

### A：非法页大小同时重置页码

- 七页都先读取原始 `page_size`；当 URL 显式给出非白名单值时，`page_size` 规范为 20 且 `page` 同时规范为 1。
- 七页各以 `page=3&page_size=499|501` 覆盖，共 14 个场景；此前实现阶段已取得 14 个预期 RED。本轮 focused 8 文件验证这些场景继续 GREEN。

### B：等待真正的最后 GET

- RabbitMQ 与 Redis 的完整 GET 参数断言均位于 `waitFor` 内，不再只等待任意旧请求后立即读取请求数组。
- 可逆 RED：临时把 `requests.at(-1)` 变异为 `requests.at(0)`，两页定向测试均因读到原始 `page=3,page_size=20` 等旧状态而失败。
- GREEN：恢复 `requests.at(-1)` 后，相同两页定向测试 2/2 通过。

### C：合法 500 URL 的真实多页恢复

- 七页 fixture 均按请求回显 `page` 与 `page_size`；500 场景固定返回脱敏最小列表和 `total=1001,total_pages=3`，不使用会被规范为单页的伪证。
- 七页分别从 `page=3&page_size=500` 恢复，先断言第 3/3 页、下拉值 500 和初始 GET，再执行一个代表性搜索、筛选或排序变更，断言 `page=1`、`page_size=500` 与最后 GET 全参数一致。
- 可逆 RED：临时从七页 `pageSizes` 移除 500，七个定向场景 7/7 失败；恢复 `[20, 50, 100, 500]` 后相同场景 7/7 通过。

### D：runtime 校验边界

- 未为 Host、Disk、MySQL、Redis 新增完整 runtime validator，也未做响应架构重构。
- Elasticsearch、RabbitMQ、Java 继续使用既有 validator；Host、Disk、MySQL、Redis 继续使用现有 typed response path。本轮只扩充分页 URL、请求时序和 fixture 证据。

## 验证

```bash
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run -- src/components/ListPage.test.tsx src/features/hosts/HostListPage.test.tsx src/features/disks/DiskPage.test.tsx src/features/mysql/MySQLPage.test.tsx src/features/redis/RedisPage.test.tsx src/features/elasticsearch/ElasticsearchPage.test.tsx src/features/rabbitmq/RabbitMQPage.test.tsx src/features/java/JavaPage.test.tsx && npm run typecheck'
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/npm-cache -v "$PWD:/src" -v /src/web/node_modules -w /src/web node:22-alpine sh -c 'npm ci --ignore-scripts >/dev/null && npm run test:run && npm run typecheck'
git diff --check
```

结果：定向 8 文件/216 项测试、全量前端 17 文件/344 项测试、typecheck 和 whitespace 检查均通过。全量 Vitest 中 Java/RabbitMQ POST 未匹配 MSW 的 stderr 是既有只读拒绝契约，测试通过。

## 提交

- Review fix round1 仅暂存七个 Page、七个对应测试和本报告，本地提交 `fix: normalize 500-row list URLs`。
- 不包含已有的 `task-2-report.md` 修改；不 push、不部署、不重启。

## Review fix round2

### 七页 UI 切换 500

- Host、Disk、MySQL、Redis、Elasticsearch、RabbitMQ、Java 均由测试实际操作“每页数量”下拉选择 500；每页都在同一个 `waitFor` 中断言 URL 回到 `page=1`、保留 `page_size=500`，并断言最后一次 GET 的完整白名单参数。
- Disk 用例继续用 fake timers 精确验证 300ms 搜索防抖；排空业务计时器后恢复 real timers，再用 `waitFor` 等待最终 URL 与 GET，避免测试轮询被 fake timers 阻塞。
- 有效 mutation RED：临时让共享 `ListPageSizeField` 的 `onChange` 忽略值 500，七页定向用例 7/7 失败；恢复正常 `onChange` 后相同集合 7/7 通过。一次更早的诊断误改了无关的 `ListSelectField`，结果不计为 RED，随即恢复且未留下差异。

### 精确 501/2 边界

- MySQL 独立用例从 `page=3&page_size=20` 实际选择 500；fixture 精确返回 `total=501,total_pages=2`，UI 显示“第 1 / 2 页，共 501 个实例”，“下一页”保持可用，点击后 URL 与最后 GET 均为 `page=2&page_size=500`。
- round1 七页 `total=1001,total_pages=3` 的 page 3 URL 恢复证据全部保留。
- mutation RED：临时将该用例的 500 行响应总数从 501 改为 500，用例因不存在第 2 页而失败；恢复 501 后单用例 1/1 通过。

### history back/forward

- Host 代表性用例先从 20 实际切到 500，再切到 50；随后调用真实 `window.history.back()` 恢复 500，再调用 `window.history.forward()` 恢复 50。四个状态均在 `waitFor` 内同时断言 URL 与最后 GET。
- mutation RED：临时把用户分页参数写入由 push 改成 `replace`，`back()` 后仍停留在 50，测试失败；恢复 push 后单用例 1/1 通过。服务端页码规范化分支继续使用原有 `replace`，无生产代码持久差异。

### Round2 验证

- 七页 UI 选 500 定向集合：7/7 通过。
- Node 22 focused：8/8 文件、222/222 测试通过；typecheck 通过。
- Node 22 全量前端：17/17 文件、350/350 测试通过；typecheck 通过。Java/RabbitMQ POST 未匹配 MSW 的 stderr 仍为既有只读拒绝契约，测试通过。
- round2 持久差异仅为七个页面测试和本报告；共享控件及页面产品代码的 mutation 均已恢复。已有 `task-2-report.md` 修改不纳入本轮提交；不 push、不部署、不重启。
