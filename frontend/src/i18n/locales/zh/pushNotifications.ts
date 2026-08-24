export default {
  pushNotifications: {
    subscription: {
      title: '浏览器通知',
      enable: '开启通知',
      disable: '关闭通知',
      status: {
        unsupported: '当前浏览器不支持',
        denied: '通知权限已被阻止',
        disabled: '未开启',
        enabled: '已开启',
      },
      description: {
        unsupported: '当前浏览器无法使用 Web Push 通知。',
        denied: '请在浏览器的网站权限设置中重新允许通知。',
        disabled: '开启后，即使没有打开网页也能收到重要通知。',
        enabled: '关闭网页后仍可接收管理员发送的重要通知。',
      },
      enabledMessage: '浏览器通知已开启',
      disabledMessage: '浏览器通知已关闭',
      failed: '更新浏览器通知设置失败',
    },
  },
}
