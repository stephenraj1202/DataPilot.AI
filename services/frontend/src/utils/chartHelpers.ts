export const CHART_COLORS = [
  '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6',
  '#06b6d4', '#f97316', '#84cc16', '#ec4899', '#6366f1',
]

export function getChartColor(index: number): string {
  return CHART_COLORS[index % CHART_COLORS.length]
}

export function buildPieData(labels: string[], data: number[]) {
  return labels.map((label, i) => ({
    name: label,
    value: data[i],
    fill: getChartColor(i),
  }))
}
