import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'
import contentSafetyBilling from './contentSafetyBilling'
import plugins from './plugins'
import wechatGroupQR from './wechatGroupQR'
import appCatalog from './appCatalog'
import iqDetection from './iqDetection'
import pushNotifications from './pushNotifications'
import menuManagement from './menuManagement'

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
  ...contentSafetyBilling,
  ...plugins,
  ...wechatGroupQR,
  ...appCatalog,
  ...iqDetection,
  ...pushNotifications,
  ...menuManagement,
}
