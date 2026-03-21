import {
  LineChart, Line, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
} from 'recharts'
import { CHART_COLORS } from '../../utils/chartHelpers'
import { formatCurrency } from '../../utils/formatters'

interface CostChartProps {
  type: 'line' | 'bar' | 'pie'
  data: Array<Record<string, unknown>>
  title: string
  dataKey?: string
  nameKey?: string
  valueKey?: string
}

export default function CostChart({ type, data, title, dataKey = 'cost', nameKey = 'name', valueKey = 'value' }: CostChartProps) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <h3 className="mb-4 text-sm font-semibold text-gray-700 dark:text-gray-300">{title}</h3>
      <ResponsiveContainer width="100%" height={240}>
        {type === 'line' ? (
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" opacity={0.2} />
            <XAxis dataKey={nameKey} tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `$${v}`} />
            <Tooltip formatter={(v: number) => formatCurrency(v)} />
            <Legend />
            <Line
              type="monotone"
              dataKey={dataKey}
              stroke={CHART_COLORS[0]}
              strokeWidth={2}
              dot={false}
              animationDuration={300}
            />
          </LineChart>
        ) : type === 'bar' ? (
          <BarChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" opacity={0.2} />
            <XAxis dataKey={nameKey} tick={{ fontSize: 11 }} />
            <YAxis tick={{ fontSize: 11 }} tickFormatter={v => `$${v}`} />
            <Tooltip formatter={(v: number) => formatCurrency(v)} />
            <Legend />
            <Bar dataKey={dataKey} fill={CHART_COLORS[0]} animationDuration={300} radius={[4, 4, 0, 0]} />
          </BarChart>
        ) : (
          <PieChart>
            <Pie
              data={data}
              dataKey={valueKey}
              nameKey={nameKey}
              cx="50%"
              cy="50%"
              outerRadius={90}
              animationDuration={300}
              label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
            >
              {data.map((_, i) => (
                <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
              ))}
            </Pie>
            <Tooltip formatter={(v: number) => formatCurrency(v)} />
          </PieChart>
        )}
      </ResponsiveContainer>
    </div>
  )
}
