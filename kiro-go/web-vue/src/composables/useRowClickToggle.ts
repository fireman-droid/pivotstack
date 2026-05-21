import { type Ref } from 'vue'
import { type DataTableRowKey } from 'naive-ui'

/**
 * 让 NDataTable 行点击任意位置都能勾上 / 取消。
 *
 * 排除以下元素的点击不触发选中：
 * - button / a 链接
 * - 表单控件（input / textarea / select / NSwitch / NInputNumber 等）
 * - selection 列本身（naive-ui 已自动处理）
 * - popconfirm / dropdown / popover 的弹层元素
 *
 * 用法：
 * ```ts
 * const checkedRowKeys = ref<DataTableRowKey[]>([])
 * const rowProps = useRowClickToggle(checkedRowKeys, row => row.id)
 * ```
 * ```vue
 * <n-data-table v-model:checked-row-keys="checkedRowKeys" :row-props="rowProps" ... />
 * ```
 */
export function useRowClickToggle<T>(
  checkedRowKeys: Ref<DataTableRowKey[]>,
  getRowKey: (row: T) => DataTableRowKey,
) {
  const INTERACTIVE = [
    'button',
    'a',
    'input',
    'textarea',
    'select',
    '.n-switch',
    '.n-input',
    '.n-input-number',
    '.n-base-selection',
    '.n-checkbox',
    '.n-radio',
    '.n-popconfirm',
    '.n-popover',
    '.n-tag',
    '.n-data-table-td--selection',
  ].join(', ')

  return (row: T) => ({
    style: 'cursor: pointer',
    onClick: (e: MouseEvent) => {
      const t = e.target as HTMLElement
      if (!t || t.closest(INTERACTIVE)) return
      const key = getRowKey(row)
      const arr = checkedRowKeys.value
      const idx = arr.indexOf(key)
      if (idx >= 0) checkedRowKeys.value = arr.filter(k => k !== key)
      else checkedRowKeys.value = [...arr, key]
    },
  })
}
