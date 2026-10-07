# 2026-10-07 未恢复功能探测记录

## 范围与证据边界

本轮目的为审计当前代码、拆分新版与旧版养成实现。用户确认战斗、装备、岛屿、邮件等尚未恢复内容应先记录，留作后续恢复参考，本轮不扩展施工。

固定基线为 `fe133b24607e0fcf29c974c4fbbc23eba090d068`。两套分支分别运行 `go test ./... -count=1`，使用独立养成测试数据库和隔离 schema；没有使用玩家库执行测试。下面是自动测试观察，不是实机玩法验收，失败也不自动证明处理器有错：可能涉及测试配置不全、旧预期与当前客户端协议不符或实现未恢复。后续必须先对照当前客户端、实际本地配置和存档再定位。

四个代表失败在未修改基线的独立工作树中再次复现：章节补给、岛屿采集、后台赠舰、邮件领奖。其他条目只记录全量结果，不虚称已逐项基线验证。

## 自动测试失败清单

| 测试 | 观察到的失败 | 基线核验 |
| --- | --- | --- |
| `TestFinishStageResolvesVirtualAwardDrops` | battle_session_test.go:433: finish stage failed: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestChapterOpSupplyUpdatesPersistedAmmoAndCell` | chapter_op_test.go:217: expected depleted supply cell item_id 0, got 5 | 独立基线复现 |
| `TestChapterTrackingIncludesLandbaseItemIDForAttachLandbase` | chapter_tracking_test.go:223: expected landbase [5,10] item_id 101, got 0 | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestClaimCollectionAwardConcurrentClaimDoesNotDuplicateReward` | collection_award_claim_test.go:210: expected exactly one success result, got 5 / 5 | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestMetaCharacterUnlockShipIdempotent` | meta_character_packets_test.go:125: expected successful unlock with ship | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestMetaCharacterUnlockShipLegacyIdempotent` | meta_character_packets_test.go:239: expected successful unlock with ship | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestShipyardModAndPursue` | shipyard_blueprint_packets_test.go:128: seed owned ship failed: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestShipyardFinishBlueprint` | shipyard_blueprint_packets_test.go:198: expected finish success with ship | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestSupportShipRequisitionSuccess` | support_ship_requisition_test.go:40: expected result 0, got 1 | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestEquipToShipEquipAndUnequip` | equip_to_ship_test.go:66: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestTransformEquipmentOnShipSuccessAllowed` | transform_equipment_on_ship_test.go:153: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestTransformEquipmentOnShipFailsWrongUpgradePath` | transform_equipment_on_ship_test.go:195: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestTransformEquipmentOnShipFailsInsufficientGold` | transform_equipment_on_ship_test.go:239: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestTransformEquipmentOnShipMovesToBagWhenForbidden` | transform_equipment_on_ship_test.go:278: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestTransformEquipmentOnShipFailsInsufficientMaterial` | transform_equipment_on_ship_test.go:318: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestUpdateShipEquipmentSkinSuccessPersistClearAndIdempotent` | update_ship_equipment_skin_test.go:52: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestUpdateShipEquipmentSkinPreservesEquipID` | update_ship_equipment_skin_test.go:159: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestUpdateShipEquipmentSkinValidationFailures` | update_ship_equipment_skin_test.go:201: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestUpdateShipEquipmentSkinValidatesAgainstEquippedItemType` | update_ship_equipment_skin_test.go:279: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestIslandEnterMapAndSyncControl` | island_new_packets_test.go:122: map enter failed: proto: required field belfast.PB_ISLAND_WILD_GATHER.refresh_time not set | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestIslandWildGatherSign` | 缺失 refresh_time 导致应答发送失败；测试继续读取不存在的帧，触发 index out of range | 独立基线复现 |
| `TestPlayerGiveItemShipMail` | player_api_test.go:173: expected 200, got 500 | 独立基线复现 |
| `TestCompensationCollectAttachmentsAndSummary` | compensation_extra_test.go:75: collect attachments: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |
| `TestMailCollectAttachments` | mail_test.go:169: collect attachments: db: not found | 独立基线复现 |
| `TestCommanderAddShipUsesConfigDrivenDefaultEquipmentSlots` | players_extra_test.go:155: add ship: db: not found | 仅在两个恢复分支全量测试中观察；未单独基线复现 |

## 后续查证入口

- 章节：补给后格子 item_id 未清零、陆基挂载 item_id 为 0；核对补给扣除与格子持久化，以及当前地图回复字段。战斗掉落的 `db: not found` 先定位缺失配置来源。
- 收藏领奖：并发两次均返回失败 5，未达到“一次成功一次拒绝”的预期；核对奖励配置完整性后再判断事务防重，不能仅改 Result。
- 舰船生成、META、蓝图、支援、装备、邮件：大量失败发生在 AddShip 的 `db: not found`，应先查默认装备/舰船配置依赖，避免给每个上层处理器各做一个空成功补丁。
- 岛屿：真实 protobuf 中野外采集/采集推送要求 `refresh_time`；测试的配置和构造器未满足必填字段，发送失败后测试又读取第二帧导致越界。后续须查证刷新时间的客户端语义，不能填任意值只求能封包。
- API 赠舰：give-item 成功，give-ship 返回 500；与 AddShip 缺配置是否同源仍待追踪，不能宣称已经定位。

## 当前工作区中暂未上传的变化

- 舰船列表切片实现存在空船坞不发送 SC12001、首批 101 项与后续步进 100 导致重复风险，应按实际协议重查。
- 活动列表的未知配置过滤仅处理 FinishTime=0，其他状态仍可能发出未知活动，尚未形成完整实测验收。
- 本机 server.toml、gateway.toml、版本数据文件、玩家数据、提取的配置和备份不上传。
- 身份验证、舰船婚约字段、技能、活动、社交和任务等其他未提交变更本轮保留于原工作区，未与养成恢复打包。已查证的动态聊天回复重载修复单独进入公共提交。

## 养成未完成语义

新版：S03 的 20 种条件仅恢复 5 种；S04 复杂链、active benefit、额外奖励、困难/无尽未完成；S05 收益节点和复杂分支未完成；S06 塔罗及完整触发/效果未完成；S07 其他地图、话题、奖励未完成；S08～S10 仍待恢复。旧评估、话题选择、结局、重置、旧地图写入等占位逻辑会与恢复状态机冲突，拆分版已关闭这些入口，等待逐项查证后再实现。

旧版：L02 跨心情倍率、分数取整和事件奖励未完成；L03 随机池、条件分支、折扣、特殊/心事奖励、目标选择完整限制、完整好感/人格成长和领奖实机仍未完成；L04 与 A01 未完成。特殊奖励不能借用主港资源 helper，养成与主港收益必须有实际协议证据。

版本拆分解决引擎共存和已知占位写入问题，不等价于这些功能已恢复。两个版本的 Windows 编译与专项测试结果另见各分支 `docs/educate-release.md`；没有拆分后部署和实机验收，不声明完整稳定版。
