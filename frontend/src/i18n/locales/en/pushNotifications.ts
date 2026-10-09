export default {
  pushNotifications: {
    subscription: {
      title: 'Browser Notifications',
      enable: 'Enable',
      disable: 'Disable',
      status: {
        unsupported: 'Not supported',
        denied: 'Permission blocked',
        disabled: 'Off',
        enabled: 'On',
        unknown: 'Unknown',
      },
      description: {
        unsupported: 'This browser does not support Web Push notifications.',
        denied: 'Allow notifications again in this site’s browser permissions.',
        disabled: 'Enable notifications to receive important updates while the website is closed.',
        enabled: 'Important administrator messages can arrive while the website is closed.',
      },
      enabledMessage: 'Browser notifications enabled',
      disabledMessage: 'Browser notifications disabled',
      failed: 'Failed to update browser notification settings',
    },
    tour: {
      title: 'Turn on browser notifications',
      enter: 'Use the switch in the account menu to get reminders when a lottery starts, when it draws, and when a task reward arrives.',
      join: 'Notifications are off. Turn on this switch to get reminders when a lottery starts, draws, or you win.',
      mute: "Don't remind me again",
      qqTitle: 'Join the QQ group',
      qq: 'Join the QQ group to get the latest news in time.',
    },
  },
}
