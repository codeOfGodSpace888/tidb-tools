用户使用手册
===

### 介绍

DM (Data Migration) 是基于 mydumper / loader / syncer 的调度管理一体化工具产品，设计的主要目的是
   - 标准化 （e.g. 工具运行，错误定义）
   - 降低运维使用成本
   - 简化错误处理流程
   - 提升产品使用体验

### 架构图

   ![DM structure](./architecture.png)

### 组件功能

#### dm-master

- 保存 DM 集群的拓扑信息
- 监控 dm-worker 进程的运行
- 监控数据同步任务的运行状态
- 提供数据同步任务管理的统一入口
- 协调 sharding 场景下各个实例的分表 DDL 同步

#### dm-worker

- binlog 的本地持久化保存
- 保存数据同步子任务的配置信息
- 编排数据同步子任务的运行
- 监控数据同步子任务的运行状态

#### dmctl

- 创建 / 更新 / 删除数据同步任务
- 查看数据同步任务状态
- 处理数据同步任务错误
- 校验数据同步任务配置的正确性


下面从各个方面详细介绍这个工具的使用方式。

### 快速开始

1. 阅读并且了解 [使用限制](./limits.md) 文档
2. 阅读并且了解 [[配置文件](https://docs.google.com/document/d/1D5qaUNcaxr441adZKyo7QzXo1J_dDlPl1N5ITlnHIQc/edit#heading=h.m7nrdrxv91e3)] 章节
3. 阅读并且了解 [[同步功能介绍](https://docs.google.com/document/d/1D5qaUNcaxr441adZKyo7QzXo1J_dDlPl1N5ITlnHIQc/edit#heading=h.kb6bkf32ww8v)] 章节
4. 根据 [运维管理/DM Ansible 运维手册] 文档部署和管理 DM 集群
  1. 部署 DM 集群组件（包括 dm-master、dm-worker、dmctl）
  2. 部署监控组件（包括 prometheus、grafana、alertmanager）
  3. 启动集群
  4. 关闭集群
  5. 升级组件版本
5. 根据下文配置文件的  [配置文件/Task 配置生成] 生成数据同步任务配置 `task.yaml`
6. 学习 [任务管理] 章节来管理和查看任务的运行
7. 更加详细的样例可以参考 [同步示例/一个数据同步任务示例] 文档

### 配置文件

#### DM 进程配置文件介绍

1. `inventory.ini` - ansible 部署配置文件，需要用户根据自己的机器拓扑进行编辑。 详情见 [运维管理/DM Ansible 运维手册]文档
2. `dm-master.toml` - dm-master 进程运行的配置文件，包含 DM 集群的拓扑信息， MySQL instance 和 dm-worker 的对应关系 （必须是一对一的关系）
3. `dm-worker.toml` - dm-worker 进程运行的配置文件，包含访问上游 MySQL instance 的配置信息

#### Task 生成

##### 配置文件

如果您使用 ansible 安装，你可以在 `<path-to-dm-ansible>/conf` 找到下面任务配置文件样例

- `task.yaml.exmaple` -  数据同步任务的标准配置文件（一个特定的任务一个 `task.yaml`）配置项解释见  [配置文件/Task 配置文件介绍] 文档
- `dm.yaml.example` - 一种简易数据同步任务配置文件，适用于分库分表配置规则比较复杂的场景，可以用来生成 `task.yaml`，配置项解释见 [配置文件/Task 简易配置文件] 文档

##### 同步任务生成

- 直接基于 `task.yaml.example` 样例文件
 - copy `task.yaml.example` 为 `your_task.yaml`
 - 参照 [配置文件/Task 配置文件介绍]文档， 修改 `your_task.yaml` 的配置项
- 通过 task 简易配置文件 `dm.yaml.example` 生成 `task.yaml` （适用于分库分表配置规则比较复杂的场景）
 - copy `dm.yaml.example` 为 `your_dm_task.yaml`
 - 参考 [配置文件/Task 简易配置文件]文档，修改 `your_dm_task.yaml` 的配置项
 - 使用 dmctl [任务管理/dmctl 使用手册] 运行  
    `generate-task-config your_dm_task.yam your_task.yaml`

修改或生成完 `your_task.yaml` 后，通过 dmctl 继续创建您的数据同步任务，参考 ·[任务管理] 章节

##### 关键概念

| 概念         | 解释                                                         | 配置文件                                                     |
| ------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| instance-id  | 唯一确定一个 MySQL / MariaDB 实例（ansible 部署会用 host:port 来组装成该 ID） | dm-master.toml 的 mysql-instance; dm.yaml 的 instance-id; task.yaml 的 instance-id |
| dm-worker ID | 唯一确定一个 dm-worker （取值于dm-worker.toml 的 worker-addr 参数） | dm-worker.toml 的 worker-addr; dmctl 命令行的 -worker/-w flag  |

mysql-instance 和 dm-worker 必须一一对应

### 任务管理

#### dmctl 管理任务

使用 dmctl 可以完成数据同步任务的日常管理功能，详细解释见 [任务管理/dmctl 使用手册]

- [创建数据同步任务]   
    示例：使用 dmctl 运行 `start-task ./your_task.yaml`
- [停止数据同步任务]  
    示例：使用 dmctl 运行 `stop-task task-name`
- [查询数据同步任务状态]  
    示例：使用 dmctl 运行 `query-status`
- [更新数据同步任务]  
    示例：编辑 `task.yaml`  
        使用 dmctl 运行 `update-task ./task.yaml`
- [暂停数据同步任务]  
    示例：使用 dmctl 运行 `pause-task task-name`
- [重启数据同步任务]  
    示例：使用 dmctl 运行 `resume-task task-name`

### 同步功能介绍

#### schema / table 同步黑白名单

上游数据库实例表的黑名过滤名单规则。过滤规则类似于 MySQL replication-rules-db / tables, 可以用来过滤或者只同步某些 database 或者某些 table 的所有操作。详情见 [配置文件/Task 配置项介绍]

#### Binlog Event 过滤

比 schema / table 同步黑白名单更加细粒度的过滤规则，可以指定只同步或者过滤掉某些 database 或者某些 table 的具体的操作，比如 `INSERT`，`TRUNCATE TABLE`。详情见  [配置文件/Task 配置项介绍]

#### column mapping 过滤

可以用来解决分库分表自增主键 ID 的冲突，根据用户配置的 instance-id 以及 schema / table 的名字编号来对自增主键 ID 的值进行改造。详情见  [配置文件/Task 配置项介绍]

#### 分库分表支持

DM 支持对原分库分表进行合库合表操作，但需要满足一些限制，详情见 [分库分表]

####  TiDB 不兼容 DDL 处理

TiDB 当前并不兼容 MySQL 支持的所有 DDL，已支持的 DDL 信息可参见: <https://github.com/pingcap/docs-cn/blob/master/sql/ddl.md>

当遇到不兼容的 DDL 时，DM 会同步报错，此时需要使用 dmctl 手动处理该错误（包括 跳过该 DDL 或 使用用户指定的 DDL 替代原 DDL），具体操作方式参见 [错误处理/skip/replace 异常 SQL]

### 运维管理

- DM 监控项介绍 - [运维管理/DM 监控介绍]

- 扩充/缩减 DM 集群 - [运维管理/ 扩充/缩减 DM 集群]
