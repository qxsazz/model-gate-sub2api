# VIP staging acceptance

This feature is opt-in. The additive migration creates VIP rules, recharge events,
manual overrides and audit records with automatic benefits disabled.

User page: `/vip`. Admin page: `/admin/vip`.

Default tiers use confirmed recharge USD 100/300/600/1500/3000,
concurrency 8/12/16/20/30, and referral percentages 2/4/6/8/10.
Group discounts and exclusive grants are configured per environment by group ID.
Private groups zth-plus/zth-pro, testing groups, and subscription groups must not
be automatically granted. Existing manual group and price overrides are preserved.

Online payments snapshot explicitly configured currency conversion and referral
percentages. Unsupported currencies block new orders only when VIP is enabled;
configure the actual USD conversion before enabling payment tests. No production
conversion rate is guessed. Historical payments are not automatically backfilled.
An audited opening record is available once per user after reviewing historical
records. It does not change the balance. Stage demo users can use these records.

Admin balance adjustments atomically append recharge changes. Payment completion
and confirmed refund triggers append idempotent principal changes. Confirmed
refunds reverse order referral credits; unrecovered commissions become debt and
offset future referral accrual. Admin recharge commissions respect the existing
affiliate switch, freeze period, duration, and cap.

VIP authentication and rates read current effective rights rather than mutating
the original auth cache or stored manual rates. This favors prompt refund
revocation; query/cache performance must be measured before a large production
rollout. RPM targets do not replace existing unlimited defaults or hard channel
limits. The interface distinguishes RPM and concurrent requests.

Acceptance: tier boundaries, duplicate callbacks, manual overrides, private-group
exclusion, partial/full refunds, referral recovery, existing Key grants, media
pricing exclusion, desktop/mobile, dark mode and precise small multipliers.

The reference site's authenticated page was not accessible during design. The
badge implements the approved Model-Gate dark/champagne style, not copied assets.

Production remains disabled until an explicit production release and configuration
review. This PR targets staging; merging to main is a separate action.

## VIP 鉴权与计费发布保护

VIP 关闭时，鉴权仅查询启用状态，缓存已确认的关闭状态最多 1 秒；本实例保存 VIP 配置会立即失效该缓存。未知状态不当作关闭。VIP 开启后的授权每个请求重新读取，不缓存成长权限；鉴权专用快照不加载展示流水。跨实例关闭状态缓存最多延迟 1 秒开放新权益，不无限保留旧授权。

人工倍率优先于 VIP 优惠，严格价格解析保留缓存与 singleflight；缓存区分人工倍率和无人工覆盖，旧基础价缓存不能覆盖新 VIP 优惠。VIP 关闭的请求不再查询 VIP 价格。未知价格返回 503，不偷偷改为基础价。

请求在预留余额之前固定价格，即使预留开关关闭也先校验。API Key 的私有请求快照与 context 供预扣、调度和异步结算复用，不进入共享鉴权缓存；订阅分组保留独立价格和人工覆盖，不套用普通 VIP 减免。结算没有快照且无法确认价格时不写入猜测金额。

显式兜底分组在转发前确认新的计费分组价格，不修改原分组快照；复合路由仍保留父分组计费。WebSocket 每个逻辑轮次重新检查身份、组权限和 VIP 权益，固定该轮价格与并发额度；异步任务捕获该轮 Key 快照，不沿用建连时的等级，也不被下一轮覆盖。

新增回归覆盖关闭时不加载完整权益、配置失效、退款撤权、人工价优先、旧缓存不覆盖 VIP、请求内价格一致、503 协议包装与禁用预留时依然阻止未知价格。此补丁不执行历史回填、不改生产分组映射、不启用生产 VIP。

## 0.075 减免、成长加赠与签到试点

迁移 246 将普通组原满档 0.1 的减免按 75% 缩放为 0.015/0.03/0.045/0.06/0.075，保留小倍率组原规则、私人排除和原倍率下限，不启用关闭中的 VIP、不修改旧订单或余额。新配置和有效自动减免上限为 0.075，人工组定价仍优先。

新字段 `recharge_bonus_enabled` 与各档 `recharge_bonus_percent` 控制未来在线余额充值：普通会员 0%、VIP1–5 为 1%–5%，以下单时实际成长等级而非徽章为准。订单再次确认并保存到账倍率、等级、本金和加赠金额；checkout-info 和用户配置接口返回同一规则的实际预览，不修改共享支付配置。关闭 VIP/成长加赠时沿用原支付设置；订阅及后台手动加款不自动再次加赠。

迁移 247–251 接入 `/achievements` 真实签到、收藏和探索；普通/VIP1–5 每日原额 0.01/0.05/0.10/0.25/0.50/1.00，按当天实际成长等级即时入账，已有待追回奖励先抵扣。现金默认关闭，支持全体符合资格账户或核验名单；迁移 250 取消正常账户签到的近期充值或消费要求，成就现金仍需核验近 30 日有效净充值或余额计费使用。日/月实时限额可选，不批量等待发奖；历史回填和 opening 不算成就现金的近期资格。用户可领取活动勋章的补签卡，补最近 30 天漏签并按历史权益即时补发，失败不扣卡；活动单枚不再发金额，知识/实践/篇章系列集齐分别可领 1.40/2.30/11.80 USD，篇章后两枚仍预告。管理员核验历史等级、政策与可信事件后可补签即时到账，无法还原时拒绝，同日升级或开通现金不补发，迁移不补发既有零金额签到。管理端支持人工授予、取消、恢复自动条件，取消默认仅撤徽章，可明确追回现金，终身领奖记录不重置。接口、持久化与并发验收见[成就与签到验收](../docs/achievements-checkin.md)。文档和 VIP 中心展示以 0.4 自动普通组、普通会员无加赠为基准的持续节省 4.70/9.31/13.83/18.27/22.62%，不含一次性奖励、邀请收入或签到。

普通用户原全局 5% 不被直接删改，但启用 VIP 成长加赠后会被新阶梯替代；切换前应公告和安排过渡，旧订单保留既有快照。生产仍需独立发布和分组 ID 映射；每组上游成本及倍率下限要核算，不能套用同一成本预算。

## CNY 成长换算

用户确认平台成长换算为 1 CNY = 1 USD 成长额，不作为市场汇率报价。新增迁移 244 仅在未配置 CNY 时补入 1，保留人工汇率和 VIP 启用状态。支付快照使用加赠及手续费前的本金，以十进制计算并按成长流水的八位精度保存。例如本金 100、余额加赠 5% 时，余额到账 105，成长增加 100。

迁移不修改历史订单或余额。旧订单没有 `vip_principal_usd` 快照，不会自动产生历史成长记录。若以后回填，应按订单核对本金与退款并保留订单关联，避免重复计入；仅用 `opening` 汇总初始记录不会自动补齐原订单的退款关联。生产实际配置在该版本经发布和迁移后才生效。

## VIP 奖励与展示更新

迁移 `242_vip_rewards.sql` 创建奖励存储，新增迁移 `245_vip_reward_two_percent.sql` 将尚未领取档位的奖励更新为门槛的 2%（当前 $2/$6/$12/$30/$60）。五档累计 $110，不是总充值额仅返 2%。不修改已部署迁移，不重置历史领取资格；已领取及已追回记录保留实际金额，不自动补差额。领取与余额入账原子完成，奖励不计入成长额、不产生邀请佣金。退款或后台退费跌破历史领取门槛时按实际已领取金额追回；余额不足记债务，由后续入账抵扣，永不重置领取资格。

用户新增 `/user/vip/membership`（奖励状态、脱敏荣誉席位、优惠摘要）和 `/user/vip/rewards/:level/claim`（本人领取）。荣誉席位不暴露充值金额、ID 或原始身份。旧有人工分组授权与独立倍率继续保留。

SQL 冒烟：在 BEGIN/ROLLBACK 内依次执行迁移和 `internal/repository/testdata/vip_reward_smoke.sql`，不得将测试脚本直接用于生产。

## 管理员本人领奖与错误审计

新增迁移 `243_vip_admin_reward_claim.sql`，不改写已发布的迁移 242。正常管理员与普通用户按相同累计有效充值门槛领取本人奖励；停用、删除账号仍被拒绝。唯一领取记录、退款追回及债务抵扣规则不变，不补发旧的失败请求。

领奖操作日志保留原动作 `user.vip.rewards.claim.create`，额外记录 `reward_level`、`result`、成功时的 `reward_amount` 或失败时的 `error_code`。重复领取为成功幂等响应，记录 `already_claimed` 与金额 0。未达门槛为 400，账号不可用为 403，VIP 未启用为 409，数据库／配置异常为 500；错误原因可供前端显示，内部 SQL 信息不返回客户端、不加入审计附加信息。

在事务回滚内应用截至 245 的迁移，与 `internal/repository/testdata/vip_admin_reward_smoke.sql` 验证管理员资格，执行奖励冒烟脚本验证普通用户回归。生产发布仍须另行确认。
