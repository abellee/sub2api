export default {
  wechatGroupQR: {
    title: '微信群二维码',
    description: '更新页面顶部微信群入口使用的二维码',
    upload: {
      title: '更新二维码',
      description: '上传包含完整二维码的群聊截图，系统会自动识别并截取二维码区域',
      select: '选择群聊二维码截图',
      limit: 'PNG 或 JPEG，最大 12 MB',
      sourcePreview: '待上传原图',
      action: '识别并更新',
      saving: '正在识别...',
      expiresAt: '二维码失效时间',
      expiresAtHint: '选填。留空表示长期有效，仅用于记录和提醒，不影响二维码继续展示。',
    },
    current: {
      title: '当前二维码',
      source: '来源',
      uploaded: '管理员上传',
      builtin: '内置默认图',
      updatedAt: '更新时间',
      expiresAt: '失效时间',
      expired: '已失效',
      never: '长期有效',
    },
    errors: {
      type: '仅支持 PNG 或 JPEG 图片',
      size: '图片大小不能超过 12 MB',
      load: '加载当前二维码失败',
      upload: '更新二维码失败'
    },
    success: '微信群二维码已更新'
  }
}
