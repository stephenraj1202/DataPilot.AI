import api from './api'

export interface QueryResult {
  chart_type: 'metric' | 'line' | 'bar' | 'pie' | 'table'
  labels: string[]
  data: number[]
  raw_data: Record<string, unknown>[]
  generated_sql: string
  execution_time_ms: number
  query_id?: string
  cached?: boolean
}

export interface DatabaseConnection {
  id: string
  connection_name: string
  db_type: string
  host: string
  port: number
  database_name: string
  status: string
}

export interface SchemaTable {
  name: string
  columns: Array<{ name: string; type: string; nullable: boolean }>
}

export interface QueryHistoryItem {
  id: string
  query_text: string
  generated_sql: string
  status: string
  execution_time_ms: number
  result_count: number
  created_at: string
}

export interface Bookmark {
  id: string
  title: string
  query_text: string
  generated_sql: string
  chart_type: string
  labels: string[]
  data: number[]
  raw_data: Record<string, unknown>[]
  connection_id: string
  created_at: string
}

// Normalize camelCase API response to snake_case QueryResult
function normalizeResult(data: Record<string, unknown>): QueryResult {
  return {
    chart_type: (data.chart_type ?? data.chartType ?? 'table') as QueryResult['chart_type'],
    labels: (data.labels ?? []) as string[],
    data: (data.data ?? []) as number[],
    raw_data: (data.raw_data ?? data.rawData ?? []) as Record<string, unknown>[],
    generated_sql: (data.generated_sql ?? data.generatedSql ?? '') as string,
    execution_time_ms: (data.execution_time_ms ?? data.executionTimeMs ?? 0) as number,
    query_id: data.query_id as string | undefined,
    cached: data.cached as boolean | undefined,
  }
}

export interface TrainedQuery {
  id: string
  connection_id: string
  question: string
  sql_query: string
  description: string | null
  is_active: boolean
  match_count: number
  created_at: string
  updated_at: string
}

export const queryService = {
  async executeQuery(connectionId: string, queryText: string): Promise<QueryResult> {
    const { data } = await api.post('/api/query/execute', {
      database_connection_id: connectionId,
      query_text: queryText,
    })
    return normalizeResult(data)
  },

  async getConnections(): Promise<{ connections: DatabaseConnection[] }> {
    const { data } = await api.get('/api/query/connections')
    return { connections: Array.isArray(data) ? data : (data.connections ?? []) }
  },

  async addConnection(payload: {
    connection_name: string
    db_type: string
    host: string
    port: number
    database_name: string
    username: string
    password: string
    ssl_enabled?: boolean
  }): Promise<DatabaseConnection> {
    const { data } = await api.post('/api/query/connections', payload)
    return data
  },

  async deleteConnection(id: string): Promise<void> {
    await api.delete(`/api/query/connections/${id}`)
  },

  async updateConnection(id: string, payload: {
    connection_name?: string
    host?: string
    port?: number
    database_name?: string
    username?: string
    password?: string
    ssl_enabled?: boolean
  }): Promise<void> {
    await api.put(`/api/query/connections/${id}`, payload)
  },

  async getSchema(connectionId: string): Promise<{ tables: SchemaTable[] }> {
    const { data } = await api.get(`/api/query/schema/${connectionId}`)
    return data
  },

  async getQueryHistory(limit = 20): Promise<{ queries: QueryHistoryItem[] }> {
    const { data } = await api.get('/api/query/history', { params: { limit } })
    return { queries: Array.isArray(data) ? data : (data.queries ?? []) }
  },

  // Bookmarks
  async createBookmark(payload: {
    title: string
    connection_id: string
    query_text: string
    generated_sql: string
    chart_type: string
    labels: string[]
    data: number[]
    raw_data: Record<string, unknown>[]
  }): Promise<{ id: string; title: string; created_at: string }> {
    const { data } = await api.post('/api/query/bookmarks', payload)
    return data
  },

  async getBookmarks(): Promise<Bookmark[]> {
    const { data } = await api.get('/api/query/bookmarks')
    return Array.isArray(data) ? data : []
  },

  async refreshBookmark(bookmarkId: string): Promise<QueryResult> {
    const { data } = await api.get(`/api/query/bookmarks/${bookmarkId}/refresh`)
    return normalizeResult(data)
  },

  async deleteBookmark(bookmarkId: string): Promise<void> {
    await api.delete(`/api/query/bookmarks/${bookmarkId}`)
  },

  async sendEmailReport(bookmarkId: string, recipientEmail: string): Promise<void> {
    await api.post(`/api/query/bookmarks/${bookmarkId}/send-report`, {
      recipient_email: recipientEmail,
    })
  },

  async getSuggestions(connectionId: string): Promise<Array<{ question: string; chart_hint: string }>> {
    const { data } = await api.get(`/api/query/suggestions/${connectionId}`)
    return Array.isArray(data) ? data : []
  },

  // Trained queries (admin)
  async getTrainedQueries(connectionId?: string): Promise<TrainedQuery[]> {
    const { data } = await api.get('/api/query/trained', { params: connectionId ? { connection_id: connectionId } : {} })
    return Array.isArray(data) ? data : []
  },

  async createTrainedQuery(payload: { connection_id: string; question: string; sql_query: string; description?: string }): Promise<{ id: string }> {
    const { data } = await api.post('/api/query/trained', payload)
    return data
  },

  async updateTrainedQuery(id: string, payload: { question?: string; sql_query?: string; description?: string; is_active?: boolean }): Promise<void> {
    await api.put(`/api/query/trained/${id}`, payload)
  },

  async deleteTrainedQuery(id: string): Promise<void> {
    await api.delete(`/api/query/trained/${id}`)
  },
}
