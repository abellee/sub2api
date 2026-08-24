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
  },
}
