package service

type WebhookManualScope string

const (
	WebhookManualAssets WebhookManualScope = "asset_library"
	WebhookManualTasks  WebhookManualScope = "media_tasks"
)

func (scope WebhookManualScope) Valid() bool {
	return scope == WebhookManualAssets || scope == WebhookManualTasks
}

func (scope WebhookManualScope) Label() string {
	if scope == WebhookManualAssets {
		return "素材库"
	}
	return "媒体任务"
}

func webhookManualPages(scope WebhookManualScope) []webhookManualPage {
	label := scope.Label()
	var eventRows [][3]string
	var dataRows, enumRows [][3]string
	var callbackJSON string
	var eventNotes []string
	var subscription string
	if scope == WebhookManualAssets {
		eventRows = [][3]string{
			{"asset.active", "可用", "素材首次被确认可用，状态版本为 1。"},
			{"asset.failed", "审核拒绝", "素材明确被拒绝，状态版本为 2；尚未发送的旧可用通知可能被取代。"},
		}
		dataRows = [][3]string{
			{"data.id", "string", "平台公开素材 ID；重传文件会获得新 ID。"},
			{"data.group_id", "string?", "所属素材组的公开 ID；有素材组时提供。"},
			{"data.status", "string", "Active 或 Failed，含义与枚举表一致。"},
			{"data.failure_reason", "string?", "失败原因码；仅审核拒绝且有明确原因时提供。"},
			{"data.updated_at", "integer?", "素材状态更新时间，Unix 秒；存在时间值时提供。"},
			{"data.state_version", "integer", "同一素材的状态版本：可用为 1，审核拒绝为 2。"},
		}
		enumRows = [][3]string{
			{"asset.active / Active", "事件 / 状态", "首次确认可用；不保证此后永远可用。"},
			{"asset.failed / Failed", "事件 / 状态", "上游明确拒绝；同一素材的终态。"},
			{"real_person", "失败原因", "包含真实人物相关内容。"},
			{"sensitive_content", "失败原因", "内容触发敏感内容规则。"},
			{"policy_rejected", "失败原因", "内容被上游策略拒绝。"},
		}
		callbackJSON = `{
  "id": "evt_asset_example",
  "object": "event",
  "type": "asset.failed",
  "created_at": 1791264000,
  "owner_user_id": 7,
  "data": {
    "id": "asset_example",
    "group_id": "group_example",
    "status": "Failed",
    "failure_reason": "policy_rejected",
    "updated_at": 1791264000,
    "state_version": 2
  }
}`
		eventNotes = []string{
			"素材先可用后被拒绝时，可用事件可能已发出；按素材 ID 与最大 state_version 更新状态。已发请求不能撤回。",
		}
		subscription = "素材库钩子配置可订阅 asset.active 和 asset.failed；管理员关闭本类通知后，新事件不再投递。"
	} else {
		eventRows = [][3]string{
			{"task.status_changed", "状态变更", "主异步任务首次保存或持久化状态发生变化时发送。"},
		}
		dataRows = [][3]string{
			{"data.task_id", "string", "客户端用于查询的公开任务 ID。"},
			{"data.platform", "string", "任务平台标识；视频通常为渠道类型数字字符串，Suno 为 suno。"},
			{"data.status", "string", "任务状态；完整枚举和含义见状态表。"},
			{"data.state_version", "integer", "同一任务的持久化状态版本；新任务从 1 开始，每次状态变化递增。"},
			{"data.progress", "string", "平台保存的进度文本；只更新进度不产生新状态事件。"},
			{"data.fail_reason", "string?", "平台记录失败原因时提供；超时或取消通常映射为 FAILURE。"},
			{"data.response", "JSON/null", "已保存的响应快照；无响应或省略时为 null。"},
			{"data.response_sha256", "string?", "原始响应内容摘要；用于关联，不是回调签名。"},
			{"data.response_truncated", "boolean?", "true 表示 Base64 或超过 4 KiB 的字符串已替换为省略提示。"},
			{"data.response_omitted", "boolean?", "true 表示原文超过 8 MiB 或处理后仍超过 64 KiB。"},
		}
		enumRows = [][3]string{
			{"NOT_START", "未开始", "已记录任务，但尚未提交到上游。"},
			{"SUBMITTED", "已提交", "提交请求已建立，等待后续任务状态。"},
			{"QUEUED", "排队中", "上游或平台队列等待执行。"},
			{"IN_PROGRESS", "处理中", "上游正在生成或处理。"},
			{"SUCCESS", "成功", "任务已成功完成；从 response 读取可用结果。"},
			{"FAILURE", "失败", "失败、超时或确认取消；结合 fail_reason 诊断。"},
			{"UNKNOWN", "未知", "暂时无法确认结果，应通过任务查询接口核验。"},
		}
		callbackJSON = `{
  "id": "evt_task_example",
  "object": "event",
  "type": "task.status_changed",
  "created_at": 1791264000,
  "owner_user_id": 7,
  "data": {
    "task_id": "task_example",
    "platform": "54",
    "status": "SUCCESS",
    "state_version": 3,
    "progress": "100%",
    "response": {
      "status": "succeeded",
      "content": {
        "video_url": "https://example.com/result.mp4"
      }
    }
  }
}`
		eventNotes = []string{
			"此事件覆盖主异步任务模型中的视频与 Suno；不包含 Midjourney 独立绘图任务或同步图片接口。",
			"响应是状态变化时的不可变快照。内容过大时会截断或省略，需用任务查询接口读取完整或最新结果。",
		}
		subscription = "任务日志的钩子配置接收 task.status_changed；管理员关闭本类通知后，新事件不再投递。"
	}

	return []webhookManualPage{
		{Title: label + "钩子接口手册", Sections: []webhookManualSection{
			{Title: "01 / 用途与配置", Body: []string{
				"协议版本 1.0 | HTTPS POST | JSON | 文档生成时读取当前平台品牌。",
				"字段类型后缀 ? 表示可选；没有该后缀的字段必填。时间戳单位为 Unix 秒。",
				"外部程序可将事件可靠写入自己的异步队列，再尽快返回 HTTP 2xx；业务处理不应阻塞回调。",
				"管理员开启本类钩子后，可在页面填写公网 HTTPS 接收地址；个人地址接收本账号事件，系统地址接收全部账号事件。",
				subscription,
			}},
			{Title: "事件目录", Rows: eventRows},
			{Title: "事件范围", Body: []string{
				"业务事件从订阅生效后产生，不回放历史。接收方应保存事件并通过对应业务查询接口核对最终状态。",
			}},
		}},
		{Title: "回调协议与公共字段", Sections: []webhookManualSection{
			{Title: "02 / 请求方式", Body: []string{
				"平台主动向配置的公网 HTTPS 地址发送 POST；请求体 Content-Type 为 application/json。接收端返回任意 HTTP 2xx 即表示接受。",
				"当前协议不附带签名密钥。收到状态事件后，可通过对应业务查询接口核对最新状态。3xx 不跟随重定向。",
			}},
			{Title: "HTTP 请求头", Rows: [][3]string{
				{"Content-Type", "string", "固定为 application/json。"},
				{"User-Agent", "string", "固定为 New-API/1.0。"},
				{"webhook-id", "string", "wh_ 前缀的投递 ID；同一端点重试不变，用于幂等去重。"},
				{"webhook-timestamp", "string", "本次发送时的 Unix 秒时间戳；重试时可能变化。"},
			}},
			{Title: "JSON 事件信封", Rows: [][3]string{
				{"id", "string", "evt_ 前缀的事件 ID；发往不同端点时相同。"},
				{"object", "string", "固定枚举值 event。"},
				{"type", "string", "本类业务事件类型；详见事件目录。"},
				{"created_at", "integer", "事件创建时的 Unix 秒；重试不变。"},
				{"owner_user_id", "integer", "业务事件所属用户 ID；系统地址可据此区分账号。"},
				{"data", "object", "事件类型对应的数据对象，字段见本手册的事件字段表。"},
			}},
		}},
		{Title: label + "事件字段", Sections: []webhookManualSection{
			{Title: "03 / data 参数", Rows: dataRows},
			{Title: "缺省与版本规则", Body: eventNotes},
		}},
		{Title: "事件示例与枚举", Sections: []webhookManualSection{
			{Title: "04 / 完整回调 JSON", Code: callbackJSON},
			{Title: "状态与原因枚举", Rows: enumRows},
		}},
		{Title: "可靠投递与异常处理", Sections: []webhookManualSection{
			{Title: "05 / 接收端处理顺序", Body: []string{
				"先按 webhook-id 去重并持久化事件，再尽快返回 HTTP 2xx；后续业务处理由自己的队列异步完成。",
				"按 owner_user_id 与业务公开 ID 路由，原子保存最大 state_version，避免迟到事件覆盖较新的状态。",
				"平台至少一次投递，不保证跨事件到达顺序。接收端应允许相同投递 ID 重试，并避免重复处理。",
			}},
			{Title: "接收端响应与平台重试", Rows: [][3]string{
				{"HTTP 2xx", "已接收", "平台确认投递完成，不再重试此投递。"},
				{"非 2xx", "重试", "包括 3xx；平台不跟随重定向。"},
				{"网络或超时", "重试", "单次请求最多等待 10 秒，读取响应失败也会重试。"},
				{"重试期限", "最长 72 小时", "指数退避；相邻两次重试间隔最多 12 小时。"},
			}},
		}},
	}
}
