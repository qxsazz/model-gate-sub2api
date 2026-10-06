export const activityPresentation: Record<
  string,
  { name: string; description: string; icon: string }
> = {
  'knowledge:intro': {
    name: '初识平台',
    description: '认识密钥、模型和使用记录',
    icon: 'grid',
  },
  'knowledge:protocol': {
    name: '协议与请求',
    description: '理解请求格式与响应结构',
    icon: 'terminal',
  },
  'knowledge:models': {
    name: '模型与任务',
    description: '根据任务选择合适的模型',
    icon: 'book',
  },
  'knowledge:billing': {
    name: '计费与额度',
    description: '区分余额、订阅与用量',
    icon: 'coin',
  },
  'knowledge:security': {
    name: '密钥与安全',
    description: '掌握凭据保管和安全使用',
    icon: 'shield',
  },
  'knowledge:troubleshoot': {
    name: '排障基础',
    description: '从报错与记录定位问题',
    icon: 'help',
  },
  'practice:endpoint': {
    name: '选择接入地址',
    description: '正确设置 API 接入地址',
    icon: 'terminal',
  },
  'practice:request': {
    name: '构造首次请求',
    description: '核对模型、消息和请求头',
    icon: 'document',
  },
  'practice:stream': {
    name: '处理流式响应',
    description: '区分正常结束与连接中断',
    icon: 'chart',
  },
  'practice:cost': {
    name: '核对使用成本',
    description: '根据价格与用量判断费用',
    icon: 'coin',
  },
  'practice:limit': {
    name: '控制调用节奏',
    description: '处理限流、并发与重试',
    icon: 'chart',
  },
  'practice:debug': {
    name: '完成排障演练',
    description: '选择具体问题的排查步骤',
    icon: 'help',
  },
}
