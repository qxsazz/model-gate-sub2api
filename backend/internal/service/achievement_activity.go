package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type ActivityTopic struct {
	Kind        string `json:"kind"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type ActivityQuestion struct {
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
}
type activityAnswer struct {
	ActivityQuestion
	Correct int
}
type ActivityQuiz struct {
	Topic     ActivityTopic      `json:"topic"`
	Questions []ActivityQuestion `json:"questions"`
}
type ActivityGrade struct {
	Score    int  `json:"score"`
	Passed   bool `json:"passed"`
	Replayed bool `json:"replayed"`
}

var activityTopics = []ActivityTopic{
	{"knowledge", "intro", "初识平台", "了解 Key、模型、分组与成长记录。"},
	{"knowledge", "protocol", "协议与请求", "认识 HTTP 请求、消息和响应结构。"},
	{"knowledge", "models", "模型与任务", "为任务选择模型，并正确理解上下文。"},
	{"knowledge", "billing", "计费与额度", "区分余额、订阅、充值与奖励。"},
	{"knowledge", "security", "密钥与安全", "掌握密钥保管与最小权限。"},
	{"knowledge", "troubleshoot", "排障基础", "通过状态码、日志与请求编号定位问题。"},
	{"practice", "endpoint", "选择接入地址", "用接入场景判断地址与凭据的正确搭配。"},
	{"practice", "request", "构造首次请求", "检查模型、消息、请求头和响应。"},
	{"practice", "stream", "处理流式响应", "处理增量、结束、取消与连接中断。"},
	{"practice", "cost", "核对使用成本", "根据给定价格与用量核对费用。"},
	{"practice", "limit", "控制调用节奏", "处理并发、频率限制与重试。"},
	{"practice", "debug", "完成排障演练", "从具体症状选出下一步排查动作。"},
}

func AchievementTopics() []ActivityTopic { return append([]ActivityTopic(nil), activityTopics...) }

// Versioned, server-owned question bank. Solutions are deliberately excluded
// from ActivityQuiz and never sent to the browser. Each row has three choices.
var activityRows = map[string]string{
	"knowledge:intro": `调用平台接口需要哪类凭据？|API Key|登录密码|浏览器截图
API Key 应当放在哪里执行调用？|受保护的服务端|公开网页源码|公共 Git 仓库
分组主要决定什么？|可用资源与计费规则|电脑屏幕亮度|浏览器语言
调用模型前应确认什么？|模型是否在可用列表|模型名称是否最长|聊天框背景颜色
用量记录主要用来做什么？|核对调用与费用|替代 API Key|提高电脑内存
累计充值成长应以什么为准？|有效充值本金|签到奖励金额|充值加赠金额
佩戴徽章会不会重复发奖？|不会|每换一次发一次|每天无限发
签到日期由谁确定？|平台服务器|浏览器手动输入|客户端时区猜测
收藏锁定的徽章如何解锁？|达到服务端核验条件|修改浏览器进度|多次点击领取
活动收集进度如何保存？|按账户持久保存|只保存在页面文字|刷新就清空`,
	"knowledge:protocol": `JSON 请求体通常使用哪个内容类型？|application/json|image/png|text/css
Bearer 凭据通常放在哪个请求头？|Authorization|Accept-Language|Cache-Control
POST 请求中的 JSON 必须满足什么？|有效 JSON 语法|可以随意省略引号|只能包含一个字符
HTTP 401 通常意味着什么？|认证未通过|接口一定停机|结果已成功生成
HTTP 403 通常意味着什么？|权限不足|一定是密码拼错|网络一定断开
HTTP 429 通常意味着什么？|请求受到限制|返回完整图片|模型不存在
HTTP 5xx 通常指哪类错误？|服务端错误|正常成功|用户界面颜色错误
请求超时后，能否确定服务端没有处理？|不能直接确定|一定没处理|一定处理成功
消息数组中的 role 表示什么？|消息角色|充值等级|网络延迟
流式响应的片段应如何理解？|增量输出|每片都是完整回答|每片都是新充值`,
	"knowledge:models": `选择模型首先应考虑什么？|任务需求与可用能力|名称长度|徽章颜色
上下文长度主要约束什么？|请求可容纳的内容量|账户注册天数|网络端口数
输入 Token 与输出 Token 是什么关系？|分别记录请求与生成内容|必定数量相同|永远不计费
模型支持文本是否代表一定支持图片？|不代表|一定支持|只取决于图片颜色
不同模型对同一文本的 Token 数是否总相同？|不一定|永远相同|只按字节数相同
超出上下文后应优先做什么？|缩减内容或选择适合的模型|无限重发|修改签到日期
更长输出通常对费用有什么影响？|可能增加输出费用|一定永久免费|一定退还充值
模型名称来自哪里更可靠？|平台可用模型列表|任意猜测|徽章名称
敏感数据发送给模型前应做什么？|按权限与需求最小化|全部公开发送|关闭日志就无需判断
模型输出用于关键流程时应做什么？|验证结果|无条件信任|只看输出长度`,
	"knowledge:billing": `余额与充值成长记录是否相同？|不是同一指标|永远相同|都等于签到次数
赠送额度是否一定算作充值本金？|不一定，按平台规则|一定计入|只看金额大小
一次已提交计费的请求重试记账应如何处理？|按幂等键避免重复|每次重试再扣|由前端决定扣多少
签到奖励是否增加累计充值？|不会|会翻倍增加|根据徽章等级增加
退款可能对充值荣誉产生什么影响？|降级或锁定并追回相关奖励|永远无影响|自动升到最高级
同日充值升级后签到能否再领一次？|不能|可以无限领|换设备就可以
奖励金额和实际到账为何可能不同？|存在待追回金额抵扣|网页字体变化|用户名太长
现金预算耗尽后签到记录如何处理？|仍记录签到|伪造已到账|删除历史签到
Token 成长应从什么来源核验？|成功提交的可信计费|客户端自报|失败响应的任意文字
金额奖励领取次数按什么约束？|每枚成就终身一次|每次佩戴一次|每次刷新一次`,
	"knowledge:security": `发现 Key 被公开泄露后应做什么？|撤销并轮换|继续使用|把密码也公开
多人项目分配 Key 应遵循什么？|最小权限|所有人共享管理员密钥|长期无权限限制
提交工单时如何处理 Key？|遮蔽敏感部分|粘贴完整 Key|放进标题
前端代码内硬编码私密 Key 安全吗？|不安全|安全，因为打包了|安全，因为用了深色主题
日志记录请求头时应做什么？|脱敏 Authorization|完整记录密钥|公开发送全部头
测试与生产凭据应如何管理？|分开并按环境授权|始终共用|放在截图里
访问奖励接口的用户 ID 应来自哪里？|认证会话|客户端随意填写|徽章图片文件
仅修改浏览器余额数字能改变真实账户吗？|不能|能|退出登录后能
备份密钥时应该如何存放？|受控秘密存储|公共网盘无密码|公开 README
最小权限的目的是什么？|限制误用或泄露影响|增加模型输出长度|提高签到金额`,
	"knowledge:troubleshoot": `定位失败请求首先保留什么？|时间和请求编号|完整密码|徽章图片大小
认证失败后优先检查什么？|Key 与认证头|签到连击|页面字号
模型不可用时应核对什么？|模型名称与分组可用范围|充值荣誉颜色|系统壁纸
余额不足时最有效的检查是什么？|余额与计费来源|浏览器主题|网络图标颜色
限流后优先采用哪种策略？|退避并减少并发|立刻无限重试|每毫秒换一个 Key
响应超时应记录哪些数据？|耗时、状态与请求编号|登录密码|全部用户资料
同一问题稳定复现时应如何反馈？|提供脱敏最小复现|只说坏了|粘贴所有密钥
成功状态码是否保证业务结果符合预期？|不保证，仍需检查内容|保证所有答案正确|保证费用为零
排查环境变量时应注意什么？|避免输出秘密值|全部发到公共日志|先公开再删除
上游故障与客户端参数错误应如何处理？|区分原因再处理|统一无限重试|统一充值`,
	"practice:endpoint": `SDK 的 Base URL 填成管理后台页面路径，下一步？|改用平台文档的 API Base URL|继续刷新后台|充值更多
接入地址结尾已有 /v1，SDK 又追加 /v1，怎么办？|按 SDK 要求避免重复路径|再加一个 /v1|删掉认证头
请求打到 localhost，却想访问测试服务，怎么办？|核对目标环境地址|更换徽章|清空用量记录
测试应用使用了生产 Key，应该？|换成测试环境授权的 Key|直接公开 Key|用生产管理员密码
把 API Key 填进 Base URL 字段，应该？|地址与凭据分别配置|把密码也拼在 URL|继续发送
接入报 DNS 错误，先检查？|域名与 DNS 解析|模型输出长度|签到权益
HTTPS 证书校验失败，应该？|核对证书与地址|永久关闭证书校验|更改用户名
旧客户端仍使用已停用地址，应该？|更新配置并核验文档|无限重试|换一个图案
调用地址大小写路径拼错，应该？|核对准确路径|增加并发|减少输入 Token
验证目标环境时，最可靠的是？|核对地址和环境专用配置|仅看页面颜色|仅看用户名`,
	"practice:request": `请求缺少 Authorization，应该？|补充 Bearer Key|发送登录密码|修改主题
model 字段为未列出的名称，应该？|使用可用模型列表中的名称|随机缩写|填徽章名称
请求体 JSON 出现尾部多余逗号，应该？|修正 JSON 语法|增加重试次数|删除认证头
消息数组为空且接口要求消息，应该？|按接口结构提供消息|充值后再发送空数组|改用管理员 ID
请求把 messages 写成 message，应该？|按对应协议修正字段|修改浏览器字体|关闭证书校验
Content-Type 为 image/png，正文却是 JSON，应该？|修正为 JSON 内容类型|删除正文|公开 Key
想知道首次调用是否成功，应该？|检查状态码与响应内容|只看按钮颜色|只看签到日期
响应返回错误对象，应该？|读取错误代码与说明|假定生成成功|重复领取奖励
调试首次请求时，应该？|先使用最小有效示例|先发全部历史记录|先开启无限并发
复制请求示例给他人前，应？|替换真实 Key 为占位符|保留真实 Key|附上登录密码`,
	"practice:stream": `流式响应只到一半，应？|区分连接中断与正常结束|当作完整结果|自动标记免费
每个增量片段到达时，应？|累积对应内容|覆盖此前全部内容|发送新充值
收到正常结束标记时，应？|完成本次流读取|立刻无限重试|删除账户
用户点击停止，应？|取消连接并处理已有结果|继续无限读取|重复扣款
等待首个片段超时，应？|记录耗时并按超时策略处理|删除认证头|改账户等级
流中返回错误事件，应？|处理错误并停止错误累积|把错误当正文成功|改网页颜色
流式结果供程序解析时，应？|完成后校验结构|每片都当完整 JSON|仅按文字长度判断
网络恢复后重发请求，应？|明确重试与幂等策略|假定先前未处理|无限重发同一付款
要支持长连接，应？|核对客户端和代理超时配置|只修改用户名|加大签到金额
收到空增量但流未结束，应？|按协议继续读取有效事件|立即伪造完成|重新创建账户`,
	"practice:cost": `输入价 $2/百万 Token，输入 100 万，输入费是多少？|$2|$20|$0.2
输出价 $4/百万 Token，输出 50 万，输出费是多少？|$2|$4|$0.5
输入费 $2、输出费 $3，总原始费用是多少？|$5|$6|$1
原始费用 $5、倍率 0.4，按该条件实际费用是多少？|$2|$5.4|$0.4
有效充值 $100、加赠 $1，按本金口径成长是多少？|$100|$101|$1
奖励 $0.10、抵扣待追回金额 $0.04，净到账多少？|$0.06|$0.14|$0.10
缓存 Token 已在互斥桶分别计数，如何求总量？|各互斥桶相加一次|再重复加到输入桶|只取最大桶
失败且没有提交计费的请求，能否自报为成长？|不能|可以填任何数量|上传截图就可以
已领取奖励 $1，换戴三次后还会新增多少奖励？|$0|$3|$1
当月预算已耗尽，界面应显示什么？|签到记录成功但奖励未发放原因|已到账但不记账|隐藏真实结果`,
	"practice:limit": `短时间收到 429，应？|降低频率并退避|立即百倍重试|删掉 API 路径
服务允许并发 8，当前同时发出 80，应？|限制并发队列|更改前端数字为 8|只换徽章
重试间隔通常应？|逐步退避并加入抖动|始终为零|一直无限增长不设上限
同一失败任务多处同时重试，应？|集中控制重试与去重|每处再增加线程|重复发奖
有 Retry-After 提示时，应？|按提示安排重试|完全忽略|当作 Key
连续服务端失败时，应？|设置重试上限并告警|无限阻塞|永久公开密钥
突发任务需要排队时，应？|设置容量与等待上限|无限堆积内存|删除计费记录
多个用户共享上游容量时，应？|兼顾分组和渠道容量约束|只看本机线程数|只看会员卡图案
临时限流解除后，应？|逐步恢复流量|瞬间发全部积压|改动余额表
统计失败率时，应？|区分错误类型和时间窗口|只数签到记录|隐藏所有失败`,
	"practice:debug": `响应 401 且 Key 已撤销，应？|更换有效授权 Key|增加输入长度|升级徽章
响应 403 且分组无权限，应？|核对授权与可用分组|关闭证书验证|无限重试
响应 404 且路径重复 /v1，应？|修正地址拼接|充值更多|更换用户名
响应 400 且 JSON 无效，应？|修正请求体|更改签到日期|调整徽章透明度
仅某模型报不可用，应？|核对该模型渠道与授权|撤销所有用户|清空全部余额
所有请求 DNS 解析失败，应？|检查域名解析与网络|提高输出 Token|增加奖励预算
请求超时但服务端有成功记录，应？|核对请求编号避免误判和盲目重试|宣布从未调用|再次重复计费
费用与预期不同，应？|核对模型价格、用量和有效倍率|只看首页余额|修改客户端进度
提交报障材料应包含什么？|脱敏请求、时间、状态和编号|完整 Key 和密码|其他用户资料
修复后应该如何验收？|复现原场景并核对实际结果|只修改页面提示|只换成浅色主题`,
}
var achievementBanks = func() map[string][]activityAnswer {
	result := map[string][]activityAnswer{}
	for key, raw := range activityRows {
		for _, row := range strings.Split(raw, "\n") {
			p := strings.Split(row, "|")
			if len(p) != 4 {
				panic("invalid activity bank " + key)
			}
			digest := sha256.Sum256([]byte(key + ":" + row))
			shift := int(digest[0]) % 3
			opts := make([]string, 3)
			for j := 0; j < 3; j++ {
				opts[(j+shift)%3] = p[j+1]
			}
			result[key] = append(result[key], activityAnswer{ActivityQuestion{p[0], opts}, shift})
		}
	}
	return result
}()

func AchievementQuiz(kind, key string) (*ActivityQuiz, error) {
	bank, ok := achievementBanks[kind+":"+key]
	if !ok {
		return nil, infraerrors.NotFound("ACTIVITY_NOT_FOUND", "活动主题不存在")
	}
	quiz := &ActivityQuiz{Questions: make([]ActivityQuestion, 0, len(bank))}
	for _, t := range activityTopics {
		if t.Kind == kind && t.Key == key {
			quiz.Topic = t
			break
		}
	}
	for _, q := range bank {
		quiz.Questions = append(quiz.Questions, q.ActivityQuestion)
	}
	return quiz, nil
}
func GradeAchievementQuiz(kind, key string, answers []int) (ActivityGrade, error) {
	bank, ok := achievementBanks[kind+":"+key]
	if !ok || len(answers) != 10 {
		return ActivityGrade{}, infraerrors.BadRequest("ACTIVITY_ANSWERS_INVALID", "请完成全部 10 个问题")
	}
	grade := ActivityGrade{}
	for i, a := range answers {
		if a < 0 || a >= len(bank[i].Options) {
			return grade, infraerrors.BadRequest("ACTIVITY_ANSWERS_INVALID", "无效的答案")
		}
		if a == bank[i].Correct {
			grade.Score++
		}
	}
	grade.Passed = grade.Score >= 8
	return grade, nil
}

type AchievementActivityRepository interface {
	SaveActivityAttempt(context.Context, int64, string, string, string, string, json.RawMessage, ActivityGrade) (json.RawMessage, error)
}

func (s *UserService) SubmitAchievementActivity(ctx context.Context, id int64, kind, key, idem string, answers []int) (json.RawMessage, error) {
	if len(idem) < 8 || len(idem) > 80 {
		return nil, infraerrors.BadRequest("ACTIVITY_REQUEST_INVALID", "无效的请求编号")
	}
	grade, e := GradeAchievementQuiz(kind, key, answers)
	if e != nil {
		return nil, e
	}
	r, ok := s.userRepo.(AchievementActivityRepository)
	if !ok {
		return nil, fmt.Errorf("activity repository unavailable")
	}
	raw, e := json.Marshal(answers)
	if e != nil {
		return nil, e
	}
	return r.SaveActivityAttempt(ctx, id, kind, key, uuid.NewString(), idem, raw, grade)
}
