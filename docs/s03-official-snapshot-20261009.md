# S03：维护后官方养成配置还原（2026-10-09）

S03仍DOING，16～19/`${num}`私有运算合同仍缺玩法结果。本轮解除最新配置无法直接取得的阻塞：官方9.7.395资源→容器→完整Lua→21张child2表→ID/字段核验→可达性审计实际跑通。无Go玩法变化、配置导入或部署。

## 输入与工具来源

北京时间20:22直接采集10800(state59,platform0)→10801，主令牌`$azhash$9$7$395$2a544b966565ede4`。清单64625行，SHA256 `ca524d46e503c961a8443a3a4fd17e517ab47f66f53755fac2d53b1ef4d053a0`；对10/8维护期间清单有13项变化，无资源路径增删。设备版本9.7.395、scripts64 MD5与本次官方资源一致。

| 输入 | 大小 | 校验 |
|---|---:|---|
| scripts64 | 39709619 | MD5 38170e48a625f12f3a89f5f5fdc754fe；SHA256 e9886cd704217a1386e7029ede658bb582f54513114f04a24ca9c1c3dc018398 |
| scripts32 | 39678108 | MD5 277ac69b98d79907050d231b43b24bae；SHA256 bb3fe3baab38a0277edfe5a5d1d10abcd6c8b4b33bcf250adf8ab261da4aaf8d |
| 既有APK metadata | 本机文件 | SHA256 e609bbed4506bbbae09a61380eb2f833a80b0b714d2936f6144cbfca2a1b7a91 |

变换参考[MrKLLM工具链](https://github.com/MrKLLM/azurlane-assetbundle-toolkit/tree/149053bb49c2e2eec9ab9f54ab25f759c1e7d4ba/tools/sharecfg_re)，字节码组件来自[Fernando2603提取器](https://github.com/Fernando2603/AzurLaneDataExtractor/tree/7e4a223845cfc3fb8cb8d0e0c5310f670c62ccc3)。它们仅为本机工具依赖，没有取其配置数据仓库。本分支不分发第三方组件、metadata和游戏原包，报告保存全部依赖哈希。

scripts32/64容器均解成UnityFS，声明长度与边界吻合，UnityPy可打开45969对象/45968容器项。字节码链采用scripts32：本次bcdec拒绝scripts64 FR2字节码，scripts32成功；不能声称两种架构均已支持。参考组件的metadata索引取钥仅在当前包正向验证，未来APK/格式变更时复验，不能当永久格式规范。

旧调查598597535/Azurlane-LuaHelper“2026-08活跃”日期不适用，本次API返回pushed_at=2018-06-22；未采用其旧包装实现作为当前成功证据。

## 完整性核验

常量扫描候选被拒绝：922条件缺param、1715效果缺condition/effect、1311列表缺content/show_content，另有一处常量数不符。行数正确或能开容器不能代替字段恢复，候选没有用于审计或导入。

完整字节码还原后，禁用文件/包/Python入口的独立Lua运行时读取真实pg.base表，验证声明all ID集合及行id。21表17626行，无增删ID；条件、效果、列表、节点、课程等逐字段一致。四处文字原始差异全部是CRLF/LF换行，归一化后21表全一致，不能称为养成内容更新。

[restore_official_educate.py](../tools/restore_official_educate.py)只输出新证据目录，不发网络请求、不写数据库/设备、不部署；报错登记status=error。bcdec输出目录须不存在，ljdec输出目录须存在，工具分别处理。当前isolated环境UnityPy1.25.4、lupa2.8。它不包含完整更新的配置入库、生效和上线流程。

```powershell
& G:/AzurLane/tmp/educate-official-20261009-env/Scripts/python.exe tools/restore_official_educate.py --capture-dir G:/AzurLane/research/educate-recovery/20261009/official-post-maintenance --metadata G:/AzurLane/tmp/apkmeta/global-metadata.dat --decoder-dir C:/Users/Administrator/.agent-reach/cache/s03-20261009 --out G:/AzurLane/research/educate-recovery/下一次独立目录 --baseline G:/AzurLane/research/educate-recovery/db-config.jsonl
python tools/educate_condition_audit.py --config-jsonl G:/AzurLane/research/educate-recovery/下一次独立目录/official-child2.jsonl --source-report G:/AzurLane/research/educate-recovery/下一次独立目录/report.json --out G:/AzurLane/research/educate-recovery/下一次独立目录/reachability.json
```

更新输入来自官方资源和本机APK，复用已安装代码，无须拉GitHub配置。两次独立Lua运行时输出的JSON键顺序可能不同，原始文件SHA各自保存；值级对照一致，不以文件字节顺序当内容变化。

## 对S03的影响与验证

官方还原快照确认1311列表根、保守可达1636效果，7/9四条件仍无可达引用。证据扩展为本次官方配置，不证明私有直接注入。审计新增`--source-report`，必须匹配validated报告和输入SHA，不能把本地或常量扫描候选标成官方来源。

16～19仍72/71/17/28行、`${num}`仍60效果。param3/30%及param300/每10资金的说明冲突未消除，窗口/绑定/取整不能从输入推出。用户已明确只有本地端和官方更新资源；无官服账号/玩法会话，未知分支保持拒绝。来源阻塞缩小，第一部分仍未整体验收。

6项工具测试通过（0.021s）：引用/未知AST、官方源SHA绑定、常量候选拒绝、all索引、原始字节无损。可复用工具实跑整链一次，21表归一化差异0。没有Go变化，不重复已通过的23项玩法回归，也不将工具测试计为玩法验收。

接口见[s03-interface-register-20261009.json](s03-interface-register-20261009.json)：U04为当前child2还原切片，U05为附来源合同的可达性审计。R1已突破当前容器/养成配置范围；全量Lua、全部配置和完整独立更新仍未验收。

原PG20:23启动，pg_ctl180秒超时后继续恢复，20:28ready，实际SQL belfast|1后启动gateway27524/belfast3792。belfast实际G:/AzurLane/bin/belfast.exe，SHA256 2A070DA304D6FD6DBF3AA0F715836F3B799DB910EDA4C5EC7A62D5A600A0318E。未删锁、重置、恢复备份、替换binary或操作玩家游戏。IP192.168.31.70，ADB只读版本/哈希。原94项修改与完整diff保留，代码只改独立新版分支。
