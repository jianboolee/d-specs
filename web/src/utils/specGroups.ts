import type { SpecItem } from '@/types/api'

export interface ItemGroup {
  /** 分组名，空字符串表示未分组 */
  name: string
  items: SpecItem[]
}

/**
 * 按 group_name 把参数项分组。
 *
 * 保持插入顺序（不排序），未分组（空 group_name）也自成一个组并保留原位置；
 * 空表时补一个未分组的占位组，让「添加参数」只有一条渲染路径。
 *
 * 画布与打印文档共用这一份实现，避免两处分组规则漂移。
 */
export function groupItems(items: SpecItem[]): ItemGroup[] {
  const map = new Map<string, SpecItem[]>()
  for (const item of items) {
    const key = item.group_name || ''
    const list = map.get(key)
    if (list) list.push(item)
    else map.set(key, [item])
  }
  return [...map.entries()].map(([name, list]) => ({ name, items: list }))
}

/**
 * 空表时补一个未分组占位组。
 *
 * 这样画布里「添加参数」按钮与新增行表单只有一条渲染路径，
 * 不必为「表里一个分组都还没有」再写一套重复的模板。
 */
export function displayGroups(items: SpecItem[]): ItemGroup[] {
  const groups = groupItems(items)
  return groups.length > 0 ? groups : [{ name: '', items: [] }]
}
