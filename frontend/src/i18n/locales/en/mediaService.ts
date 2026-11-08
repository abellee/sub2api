import help from './mediaServiceHelp'
export default {
  mediaService: {
    help,
    userId: 'User ID', queryRecords: 'Query download records', recordsHint: 'The main site stores links, owners and metadata only. Enter a user ID to view up to 200 completed records. Downloads come directly from the media service.', fileInfo: 'File information', downloadLink: 'Download link', openDownload: 'Open download', noRecords: 'No completed download records for this user.',
    title: 'Media Service', description: 'The media service stores files. The main site records user download links and handles billing.',
    preview: 'Local preview: settings are saved locally. Connection and callback checks call the local service. Real user billing is unavailable here.',
    tabsLabel: 'Configuration sections', tabs: { main: 'Main site & records', runtime: 'Service settings', rules: 'Generation rules', bindings: 'Group bindings', callback: 'Result notifications', tasks: 'Pending tasks', records: 'Download records' },
    checkConnection: 'Check connection', configured: 'Set', keepSecret: 'Leave blank to keep current value',
    mainHint: 'The main site stores links, owners and metadata. No media bytes are stored on the main site. Management connections apply immediately; restart for relay and signing settings.',
    runtimeHint: 'Files are deleted after the configured retention period. Set a public download URL for production. Retention changes apply immediately; restart for other settings. Stop and migrate existing media before changing directories.',
    mainRestart: 'Main-site restart required:', agentRestart: 'Service restart required:', activeSettings: 'Show active settings (secrets hidden)',
    saveMain: 'Save main-site settings', saveRuntime: 'Save service settings', saved: 'Saved and applied', mainSavedRestart: 'Saved. Management calls use the new connection now. Restart the main site to apply relay and signing settings.', runtimeSavedRestart: 'Saved. Restart the service to apply.',
    failed: 'Operation failed. Please retry.', connected: 'Management connection and authentication succeeded.',
    imageTemplate: 'New image rule', videoTemplate: 'New video rule', rulesHint: 'Both image and video clients receive asynchronous task IDs. This setting describes how the provider responds. Templates are starting points: adapt them to your provider. Increase the version when editing a saved rule.',
    noRules: 'No rules yet. Start with a template.', ruleId: 'Rule name', ruleVersion: 'Version', upstreamMode: 'Provider response mode', syncUpstream: 'Provider returns result directly', asyncUpstream: 'Provider returns a task ID to query',
    createPath: 'Provider generation path', createTimeout: 'Generation request timeout (seconds)', queryPath: 'Provider query path (with task placeholder)', taskIdPath: 'Task ID response field', statusPath: 'Status response field', resultPath: 'Result files response field', ruleDescription: 'Description',
    saveRule: 'Save rule', deleteRule: 'Delete rule', advancedRule: 'Full rule configuration (JSON)', advancedHint: 'All settings: request headers, field transformations, constants, upload conversion, usage extraction, success/failure states, query timing and idempotency headers. Saving here uses the full JSON below.', saveAdvanced: 'Save full JSON',
    dryRunHint: 'Request and response samples for a saved rule (does not call the provider)', dryRun: 'Run preview', invalidRuleId: 'Use letters, numbers, dots, underscores and hyphens for rule names.',
    bindingsHint: 'Bind an existing group to a saved rule. Provider URL and credentials come from group accounts. Videos use existing main-site billing; supported duration is currently 1–15 seconds. Changes apply immediately.',
    videoRule: 'Video rule (optional)', noVideoRule: 'Use the existing video service',
    groupId: 'Group ID', selectRule: 'Select a rule', estimateKind: 'Cost estimate type', image: 'Image', video: 'Video', enabled: 'Enabled', saveBinding: 'Save binding', noBindings: 'No group bindings yet.',
    yes: 'Yes', no: 'No', actions: 'Actions', edit: 'Edit', delete: 'Delete', confirmDelete: 'Delete this configuration?', deleted: 'Deleted',
    callbackHint: 'After saving files, the service notifies the main site to record user download links and bill. Confirmation enables downloads; files remain on the service. The signing key comes from main-site connection settings. Restart the main site when rotating this key. Save before testing.',
    callbackUrl: 'Full main-site notification URL', maxAttempts: 'Maximum attempts', backoff: 'Initial retry delay (seconds)', saveCallback: 'Save notifications', testCallback: 'Test notification', callbackFailed: 'Notification test failed', callbackPassed: 'Signature verified. The main site received the test notification.',
    tasksHint: 'Files stay on the service; the main site stores download records only. Attach the provider task ID or original response to resume processing. Requests are never blindly resubmitted when provider acceptance is unknown.',
    taskId: 'Task ID', taskStatus: 'Task status', callbackStatus: 'Notification status', noTasks: 'No pending tasks.', refresh: 'Refresh', recover: 'Resume processing', recoveryBody: 'Recovery JSON: use an empty object to retry, upstream_task_id to attach a task, or response to attach the original response.', recoveryStarted: 'Recovery submitted.', result: 'Operation result',
    fields: {
      enabled: 'Enable media relay', mediad_base_url: 'Service URL', management_token: 'Management token', relay_secret: 'Relay signing key', callback_secret: 'Notification signing key', bindings_path: 'Group bindings file',
      forward_timeout_seconds: 'Relay request timeout (seconds)', management_timeout_seconds: 'Management request timeout (seconds)',
      listen: 'Service listen address', data_dir: 'Task state directory', result_dir: 'Service media storage directory', download_base_url: 'Public download base URL', max_file_mib: 'Result file limit (MiB)', max_sync_seconds: 'Default generation timeout (seconds)', max_concurrent: 'Concurrent tasks', poll_interval_seconds: 'Default query interval (seconds)', max_poll_seconds: 'Default query timeout (seconds)', download_timeout_seconds: 'File download timeout (seconds)', download_attempts: 'File download attempts', file_retention_hours: 'File retention (hours; 0 keeps forever)', task_retention_hours: 'Eligible task record retention (hours)', gc_minutes: 'Task cleanup interval (minutes)', persist_credentials: 'Save credentials for restart recovery', log_level: 'Log level'
    }
  }
}
