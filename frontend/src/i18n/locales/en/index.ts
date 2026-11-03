import { mergeMessages } from '../mergeMessages'
import adminAccessKeys from './adminAccessKeys'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import restored from './restored'
import lottery from './lottery'
import manager from './manager'
import modelStatus from './modelStatus'
import userSupport from './userSupport'
import modelProbes from './modelProbes'
import categoryDefaults from './categoryDefaults'

export default mergeMessages({
  adminAccessKeys,
  userSupport,
  modelProbes,
  categoryDefaults,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
  ...lottery,
  ...manager,
  ...modelStatus,
}, restored)
