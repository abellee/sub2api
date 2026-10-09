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
        unknown: '未知',
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
    tour: {
      title: '开启浏览器通知',
      enter: '打开右上角菜单里的通知开关，即可接收抽奖开始、开奖和任务奖励提醒。',
      join: '你还没有开启通知。打开这里的开关后，活动开始、开奖和中奖时会提醒你。',
      mute: '不再提醒',
      qqTitle: '加入 QQ 群',
      qq: '加入 QQ 群可以及时获取最新消息。',
    },
  },
}
