export interface ContactChannel {
  id: string
  kind: 'qq-group' | 'wechat' | 'qq'
  labelKey: string
  value: string
}

// Public support identifiers supplied by the operator with publication consent.
// No contact URL or QR code is implied by an identifier alone.
export const contactChannels: ContactChannel[] = [
  { id: 'qq-group', kind: 'qq-group', labelKey: 'contact.qqGroup', value: '645392969' },
  { id: 'wechat-1', kind: 'wechat', labelKey: 'contact.wechat1', value: 't1783613892' },
  { id: 'wechat-2', kind: 'wechat', labelKey: 'contact.wechat2', value: 'jia940588775' },
  { id: 'wechat-owner', kind: 'wechat', labelKey: 'contact.wechatOwner', value: 'cc_model_gate' },
]
