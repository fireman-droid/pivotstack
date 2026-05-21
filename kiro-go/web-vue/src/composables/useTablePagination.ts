import { reactive } from 'vue'

/**
 * naive-ui NDataTable 用的统一分页配置。
 *
 * 功能：
 * - 总数前缀（"共 N 条"）
 * - pageSize 切换（10 / 20 / 50 / 100 / 200）
 * - 快速跳转（输入页码）
 * - 切 pageSize 时自动回到第 1 页
 * - 短列表（≤ pageSize）时仍显示总数行
 *
 * 用法：
 * ```vue
 * <script setup>
 * import { useTablePagination } from '@/composables/useTablePagination'
 * const pagination = useTablePagination(20)
 * </script>
 *
 * <n-data-table :data="rows" :pagination="pagination" />
 * ```
 */
export function useTablePagination(defaultPageSize = 20) {
  const pagination = reactive({
    page: 1,
    pageSize: defaultPageSize,
    showSizePicker: true,
    pageSizes: [10, 20, 50, 100, 200],
    // 显式关闭内部 size picker 的 loading 状态，避免 naive-ui NSelect 的 caret/loading icon a11y 误识为"image loading"
    selectProps: { loading: false },
    showQuickJumper: true,
    simple: false,
    prefix: ({ itemCount }: { itemCount: number }) => `共 ${itemCount} 条`,
    onChange: (page: number) => {
      pagination.page = page
    },
    onUpdatePageSize: (size: number) => {
      pagination.pageSize = size
      pagination.page = 1
    },
  })
  return pagination
}
