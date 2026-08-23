import type { App } from 'vue'
import { installDanQingFeedback } from '@danqing/dq-shell'
import {
  DqButton,
  DqCheckbox,
  DqDialog,
  DqDropdown,
  DqDropdownItem,
  DqDropdownMenu,
  DqEmpty,
  DqIcon,
  DqIconButton,
  DqInput,
  DqInputNumber,
  DqOption,
  DqSelect,
  DqSlider,
  DqSectionTabPanel,
  DqSectionTabs,
  DqSectionTabTrigger,
  DqSegmented,
  DqSurfaceCard,
  DqSwitch,
  DqTag,
  DqCollapse,
  DqCollapseItem,
} from '@danqing/dq-shell'

const DQ_COMPONENTS = {
  DqSectionTabs,
  DqSectionTabTrigger,
  DqSectionTabPanel,
  DqSegmented,
  DqButton,
  DqDialog,
  DqDropdown,
  DqDropdownMenu,
  DqDropdownItem,
  DqEmpty,
  DqIcon,
  DqIconButton,
  DqTag,
  DqSurfaceCard,
  DqInput,
  DqInputNumber,
  DqOption,
  DqSelect,
  DqSlider,
  DqCheckbox,
  DqSwitch,
  DqCollapse,
  DqCollapseItem,
} as const

export function installDanQingUi(app: App) {
  installDanQingFeedback(app)
  for (const [name, component] of Object.entries(DQ_COMPONENTS)) {
    app.component(name, component)
  }
}
