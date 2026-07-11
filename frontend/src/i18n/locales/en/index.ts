import { mergeMessages } from '../mergeMessages'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import admin from './admin'
import misc from './misc'
import restored from './restored'

export default mergeMessages({
  ...landing,
  ...common,
  ...dashboard,
  admin,
  ...misc,
}, restored)
