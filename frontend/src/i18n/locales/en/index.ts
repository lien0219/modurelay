import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import activity from './activity'
import distribution from './distribution'
import canvas from './canvas'
import tools from './tools'
import workspace from './workspace'
import workspaceWebhooks from './workspaceWebhooks'
import workspaceIdentity from './workspaceIdentity'
import workspaceSecurity from './workspaceSecurity'
import notifications from './notifications'
import serviceAccounts from './serviceAccounts'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
  ...activity,
  ...distribution,
  ...canvas,
  ...tools,
  workspace: { ...workspace.workspace, ...workspaceWebhooks.workspace, ...workspaceIdentity.workspace, ...workspaceSecurity.workspace },
  ...notifications,
  ...serviceAccounts,
  nav: {
    ...common.nav,
    ...tools.nav,
  },
}
