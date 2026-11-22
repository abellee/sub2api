export default {
  wechatGroupQR: {
    title: 'WeChat Group QR',
    description: 'Update the QR code shown in the WeChat group menu at the top of the page',
    upload: {
      title: 'Update QR code',
      description: 'Upload a group screenshot containing the full QR code. The QR region is detected and cropped automatically.',
      select: 'Select a group QR screenshot',
      limit: 'PNG or JPEG, up to 12 MB',
      sourcePreview: 'Original image',
      action: 'Detect and update',
      saving: 'Detecting...',
      expiresAt: 'QR code expiry',
      expiresAtHint: 'Optional. Leave empty for no expiry. This is informational only and does not hide the QR code.',
    },
    current: {
      title: 'Current QR code',
      source: 'Source',
      uploaded: 'Admin upload',
      builtin: 'Built-in default',
      updatedAt: 'Updated at',
      expiresAt: 'Expires at',
      expired: 'Expired',
      never: 'No expiry',
    },
    errors: {
      type: 'Only PNG and JPEG images are supported',
      size: 'The image must not exceed 12 MB',
      load: 'Failed to load the current QR code',
      upload: 'Failed to update the QR code'
    },
    success: 'WeChat group QR code updated'
  }
}
