package enum

func (value TaskStep) Label() string {
	switch value {
	case StepContent:
		return "拿内容和要求"
	case StepPublish:
		return "发布到指定平台"
	case StepLinks:
		return "提交发布链接"
	case StepData:
		return "提交播放数据"
	case StepSettle:
		return "奖励结算"
	default:
		return "未知步骤"
	}
}
