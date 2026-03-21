# Design Document: SaaS FinOps Analytics Platform

## Overview

The SaaS FinOps Analytics Platform is a production-grade, multi-tenant cloud cost management and AI-powered database analytics system. The platform enables organizations to connect multiple cloud providers (AWS, Azure, GCP) and external databases to gain insights through natural language queries and comprehensive cost analytics.

### System Goals

- Provide secure, multi-tenant SaaS architecture with complete data isolation
- Enable real-time cloud cost aggregation and anomaly detection across multiple providers
- Convert natural language queries to SQL for business users without technical expertise
- Support flexible subscription billing with Stripe integration
- Deliver scalable microservices architecture deployable on any cloud platform
- Ensure enterprise-grade security with JWT authentication, RBAC, and API rate limiting

### Technology Stack

**Backend Services:**
- API Gateway: Golang with Gin framework
- Auth Service: Golang with JWT and bcrypt
- Billing Service: Golang with Stripe SDK
- FinOps Service: Golang with cloud provider SDKs (AWS, Azure, GCP)
- AI Query Engine: Python with FastAPI and LangChain/OpenAI

**Frontend:**
- React 18 with TypeScript
- UI Libraries: Material UI / ShadCN / Tailwind CSS
- State Management: React Query + Context API
- Charting: Recharts / Chart.js

**Data Layer:**
- Primary Database: MySQL 8.0
- Cache: Redis 7.0
- External Databases: PostgreSQL, MySQL, MongoDB, SQL Server (customer-connected)

**Infrastructure:**
- Containerization: Docker
- Orchestration: Kubernetes
- CI/CD: GitHub Actions / GitLab CI
- Monitoring: Prometheus + Grafana


## Architecture

### High-Level Architecture

The platform follows a microservices architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (React)                         │
│              Landing │ Dashboard │ FinOps │ AI Query             │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS/REST
┌────────────────────────────▼────────────────────────────────────┐
│                      API Gateway (Golang/Gin)                    │
│         Auth Middleware │ Rate Limiter │ Request Router          │
└─────┬──────────┬──────────┬──────────┬─────────────────────────┘
      │          │          │          │
      ▼          ▼          ▼          ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────────────┐
│  Auth    │ │ Billing  │ │  FinOps  │ │   AI Query Engine   │
│ Service  │ │ Service  │ │ Service  │ │   (Python/FastAPI)  │
│ (Golang) │ │ (Golang) │ │ (Golang) │ │                     │
└────┬─────┘ └────┬─────┘ └────┬─────┘ └──────────┬──────────┘
     │            │            │                   │
     │            │            │                   │
     ▼            ▼            ▼                   ▼
┌────────────────────────────────────────────────────────────────┐
│                    Primary Database (MySQL)                     │
│  users │ accounts │ roles │ subscriptions │ cloud_costs │ ...   │
└────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  Redis Cache    │
                    └─────────────────┘

External Integrations:
┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐
│ Stripe API   │  │ Cloud APIs   │  │ External Databases       │
│              │  │ AWS/Azure/GCP│  │ PostgreSQL/MySQL/MongoDB │
└──────────────┘  └──────────────┘  └──────────────────────────┘
```

### Microservices Design

**API Gateway (Port 8080)**
- Entry point for all client requests
- JWT and API key validation
- Request routing to appropriate microservices
- Rate limiting based on subscription tier
- Circuit breaker pattern for fault tolerance
- Request/response logging

**Auth Service (Port 8081)**
- User registration and email verification
- JWT token generation (15-min access, 7-day refresh)
- Password reset flow with secure tokens
- API key generation and validation
- Password hashing with bcrypt (cost factor 12)
- Role-based access control enforcement

**Billing Service (Port 8082)**
- Stripe customer and subscription management
- Webhook handler for payment events
- Invoice generation and storage
- Plan upgrade/downgrade logic with proration
- Payment failure notifications
- Super Admin revenue routing

**FinOps Service (Port 8083)**
- Multi-cloud cost data aggregation (AWS, Azure, GCP)
- 6-hour sync intervals for cost data
- Cost anomaly detection (20% threshold)
- Optimization recommendations (idle resources, oversized instances)
- Regional cost breakdown
- 30-day moving average baseline calculation

**AI Query Engine (Port 8084)**
- Natural language to SQL conversion using LLM
- Schema extraction from external databases
- Query execution with 30-second timeout
- Result formatting for chart rendering
- Query result caching (5-minute TTL in Redis)
- Support for PostgreSQL, MySQL, MongoDB, SQL Server


### Deployment Architecture

**Containerization Strategy:**
- Each microservice packaged as independent Docker container
- Multi-stage builds for optimized image sizes
- Health check endpoints at `/health` for orchestration
- Environment-based configuration via config.ini and env vars

**Kubernetes Deployment:**
```yaml
# Example service deployment structure
- API Gateway: 3 replicas, LoadBalancer service
- Auth Service: 2 replicas, ClusterIP service
- Billing Service: 2 replicas, ClusterIP service
- FinOps Service: 2 replicas, ClusterIP service
- AI Query Engine: 3 replicas, ClusterIP service
- MySQL: StatefulSet with persistent volume
- Redis: Deployment with persistent volume
```

**Scaling Strategy:**
- Horizontal Pod Autoscaling based on CPU/memory
- API Gateway scales based on request rate
- AI Query Engine scales based on queue depth
- Database read replicas for query-heavy workloads


## Components and Interfaces

### API Gateway Component

**Responsibilities:**
- Route incoming requests to appropriate microservices
- Validate authentication tokens (JWT and API keys)
- Enforce rate limits based on subscription tier
- Implement circuit breaker for service failures
- Log all requests for audit and monitoring

**Key Interfaces:**

```go
// Middleware for JWT validation
func AuthMiddleware() gin.HandlerFunc {
    // Validate Access_Token from Authorization header
    // Extract user_id and account_id from claims
    // Attach to request context
}

// Rate limiting middleware
func RateLimitMiddleware() gin.HandlerFunc {
    // Check subscription tier from user context
    // Enforce tier-specific limits (100/500/2000/10000 req/min)
    // Return 429 if exceeded
}

// Circuit breaker for service calls
type CircuitBreaker struct {
    FailureThreshold int // 5 failures
    Timeout          time.Duration // 30 seconds
    State            string // open, closed, half-open
}
```

**Endpoints:**
- `POST /api/auth/*` → Auth Service
- `POST /api/billing/*` → Billing Service
- `GET /api/finops/*` → FinOps Service
- `POST /api/query/*` → AI Query Engine

### Auth Service Component

**Responsibilities:**
- User registration with email verification
- JWT token generation and refresh
- Password reset flow
- API key management
- RBAC enforcement

**Key Interfaces:**

```go
// User registration
POST /auth/register
Request: {
    "email": "user@example.com",
    "password": "SecurePass123!",
    "account_name": "Acme Corp"
}
Response: {
    "user_id": "uuid",
    "verification_sent": true
}

// Login
POST /auth/login
Request: {
    "email": "user@example.com",
    "password": "SecurePass123!"
}
Response: {
    "access_token": "jwt...",
    "refresh_token": "jwt...",
    "expires_in": 900
}

// Token refresh
POST /auth/refresh
Request: {
    "refresh_token": "jwt..."
}
Response: {
    "access_token": "jwt...",
    "expires_in": 900
}

// API key generation
POST /auth/api-keys
Request: {
    "name": "Production API Key",
    "expires_in_days": 365
}
Response: {
    "api_key": "32-char-secure-key",
    "warning": "Save this key, it won't be shown again"
}
```

**Password Security:**
- Minimum 12 characters
- Must contain: uppercase, lowercase, number, special character
- Reject top 10,000 common passwords
- Bcrypt hashing with cost factor 12
- Prevent reuse of last 5 passwords

### Billing Service Component

**Responsibilities:**
- Stripe customer and subscription lifecycle
- Webhook event processing
- Invoice management
- Plan enforcement

**Key Interfaces:**

```go
// Create subscription
POST /billing/subscribe
Request: {
    "plan": "pro", // free, base, pro, enterprise
    "payment_method_id": "pm_..."
}
Response: {
    "subscription_id": "sub_...",
    "status": "active",
    "current_period_end": "2024-02-01T00:00:00Z"
}

// Upgrade/downgrade plan
PUT /billing/subscription
Request: {
    "new_plan": "enterprise"
}
Response: {
    "proration_amount": 1500, // cents
    "effective_date": "immediate"
}

// Webhook handler
POST /billing/webhook
Headers: {
    "Stripe-Signature": "..."
}
Body: {
    "type": "invoice.payment_succeeded",
    "data": {...}
}
```

**Subscription Plans:**
- Free: $0/month, 30-day trial, 1 cloud account, 2 databases, 100 req/min
- Base: $10/month, 3 cloud accounts, 5 databases, 500 req/min
- Pro: $20/month, 10 cloud accounts, unlimited databases, 2000 req/min
- Enterprise: $50/month, unlimited everything, 10000 req/min


### FinOps Service Component

**Responsibilities:**
- Connect and sync cloud accounts (AWS, Azure, GCP)
- Aggregate cost data every 6 hours
- Detect cost anomalies (>20% deviation)
- Generate optimization recommendations
- Provide cost analytics APIs

**Key Interfaces:**

```go
// Connect cloud account
POST /finops/cloud-accounts
Request: {
    "provider": "aws", // aws, azure, gcp
    "credentials": {
        "access_key_id": "...",
        "secret_access_key": "..."
    },
    "account_name": "Production AWS"
}
Response: {
    "cloud_account_id": "uuid",
    "status": "connected",
    "last_sync": null
}

// Get cost summary
GET /finops/costs/summary?start_date=2024-01-01&end_date=2024-01-31
Response: {
    "total_cost": 15420.50,
    "currency": "USD",
    "breakdown_by_provider": {
        "aws": 10200.00,
        "azure": 3120.50,
        "gcp": 2100.00
    },
    "breakdown_by_service": [
        {"service": "EC2", "cost": 5000.00},
        {"service": "S3", "cost": 1200.00}
    ]
}

// Get anomalies
GET /finops/anomalies?days=30
Response: {
    "anomalies": [
        {
            "date": "2024-01-15",
            "baseline_cost": 500.00,
            "actual_cost": 850.00,
            "deviation_percentage": 70,
            "severity": "high",
            "contributing_services": ["Lambda", "DynamoDB"]
        }
    ]
}

// Get recommendations
GET /finops/recommendations
Response: {
    "recommendations": [
        {
            "type": "idle_resource",
            "resource_id": "i-1234567890",
            "description": "EC2 instance with 0% CPU for 7 days",
            "potential_monthly_savings": 150.00
        }
    ],
    "total_potential_savings": 2340.00
}
```

**Cost Sync Process:**
1. Every 6 hours, iterate through all connected cloud accounts
2. Call cloud provider APIs (AWS Cost Explorer, Azure Cost Management, GCP Billing)
3. Extract daily cost data with service, resource, region breakdown
4. Store in `cloud_costs` table
5. Calculate 30-day moving average baseline
6. Detect anomalies (>20% deviation from baseline)
7. Generate optimization recommendations

### AI Query Engine Component

**Responsibilities:**
- Convert natural language to SQL
- Extract database schemas
- Execute queries with timeout
- Format results for visualization
- Cache query results

**Key Interfaces:**

```python
# Natural language query
POST /query/execute
Request: {
    "database_connection_id": "uuid",
    "query_text": "Show me top 10 customers by revenue this month"
}
Response: {
    "chart_type": "bar",
    "labels": ["Customer A", "Customer B", ...],
    "data": [15000, 12000, ...],
    "raw_data": [...],
    "generated_sql": "SELECT customer_name, SUM(revenue) ...",
    "execution_time_ms": 245
}

# Get database schema
GET /query/schema/{connection_id}
Response: {
    "tables": [
        {
            "name": "customers",
            "columns": [
                {"name": "id", "type": "int", "nullable": false},
                {"name": "name", "type": "varchar(255)", "nullable": false}
            ]
        }
    ]
}
```

**Query Processing Flow:**
1. Receive natural language query
2. Fetch database schema from Schema_Generator
3. Construct LLM prompt with schema context
4. Generate SQL using OpenAI/LangChain
5. Validate SQL syntax
6. Check cache (key: query_text + connection_id)
7. If cache miss, execute against external database (30s timeout)
8. Determine appropriate chart type based on result structure
9. Format response with labels, data, chart type
10. Cache result for 5 minutes in Redis
11. Log query in `query_logs` table

**Chart Type Selection Logic:**
- Single numeric value → "metric"
- Time series data → "line"
- Category comparison → "bar"
- Part-to-whole → "pie"
- Tabular data → "table"


### Frontend Component

**Responsibilities:**
- User interface for all platform features
- Authentication flow
- Dashboard visualization
- Responsive design (desktop, tablet, mobile)
- Theme management (dark/light mode)

**Key Pages:**

1. **Landing Page** (`/`)
   - Marketing content
   - Feature highlights
   - Pricing table
   - Sign up CTA

2. **Authentication** (`/login`, `/register`, `/verify-email`, `/reset-password`)
   - Login form with email/password
   - Registration with email verification
   - Password reset flow
   - OAuth integration (future)

3. **Dashboard** (`/dashboard`)
   - Customizable widget layout
   - Drag-and-drop repositioning
   - Quick metrics (total cost, active queries, anomalies)
   - Recent activity feed

4. **FinOps Analytics** (`/finops`)
   - Cost summary cards
   - Monthly trend line chart
   - Service breakdown pie chart
   - Daily cost bar chart
   - Anomaly alerts
   - Optimization recommendations
   - Cloud account management

5. **AI Query Tool** (`/query`)
   - Natural language input
   - Database connection selector
   - Query history
   - Result visualization (auto-chart)
   - Export to CSV/JSON
   - Query sharing

6. **Billing** (`/billing`)
   - Current plan display
   - Usage metrics vs limits
   - Upgrade/downgrade options
   - Invoice history
   - Payment method management

7. **Settings** (`/settings`)
   - Profile management
   - Team members and roles
   - API key management
   - SMTP configuration
   - Database connections
   - Notification preferences

**State Management:**
- React Query for server state (caching, refetching)
- Context API for global state (auth, theme, user)
- Local storage for preferences

**Component Library Structure:**
```
src/
├── components/
│   ├── auth/
│   │   ├── LoginForm.tsx
│   │   ├── RegisterForm.tsx
│   │   └── PasswordReset.tsx
│   ├── dashboard/
│   │   ├── DashboardGrid.tsx
│   │   ├── Widget.tsx
│   │   └── MetricCard.tsx
│   ├── finops/
│   │   ├── CostChart.tsx
│   │   ├── AnomalyAlert.tsx
│   │   └── RecommendationCard.tsx
│   ├── query/
│   │   ├── QueryInput.tsx
│   │   ├── ResultChart.tsx
│   │   └── QueryHistory.tsx
│   └── common/
│       ├── Navbar.tsx
│       ├── Sidebar.tsx
│       └── LoadingSpinner.tsx
├── hooks/
│   ├── useAuth.ts
│   ├── useFinOps.ts
│   └── useQuery.ts
├── services/
│   ├── api.ts
│   ├── auth.service.ts
│   ├── finops.service.ts
│   └── query.service.ts
└── utils/
    ├── chartHelpers.ts
    └── formatters.ts
```


## Data Models

### Primary Database Schema (MySQL)

**users**
```sql
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email_verified BOOLEAN DEFAULT FALSE,
    verification_token VARCHAR(255),
    verification_token_expiry DATETIME,
    reset_token VARCHAR(255),
    reset_token_expiry DATETIME,
    last_password_change DATETIME,
    password_history JSON, -- Last 5 password hashes
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    INDEX idx_account_id (account_id),
    INDEX idx_email (email)
);
```

**accounts**
```sql
CREATE TABLE accounts (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    INDEX idx_name (name)
);
```

**roles**
```sql
CREATE TABLE roles (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL, -- super_admin, account_owner, admin, user, viewer
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**permissions**
```sql
CREATE TABLE permissions (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL, -- e.g., finops:read, query:execute, billing:manage
    resource VARCHAR(50) NOT NULL, -- finops, query, billing, settings
    action VARCHAR(50) NOT NULL, -- read, write, delete, execute
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**role_permissions**
```sql
CREATE TABLE role_permissions (
    role_id VARCHAR(36) NOT NULL,
    permission_id VARCHAR(36) NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (role_id) REFERENCES roles(id),
    FOREIGN KEY (permission_id) REFERENCES permissions(id)
);
```

**user_roles**
```sql
CREATE TABLE user_roles (
    user_id VARCHAR(36) NOT NULL,
    role_id VARCHAR(36) NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (role_id) REFERENCES roles(id)
);
```

**api_keys**
```sql
CREATE TABLE api_keys (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    last_used_at DATETIME,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user_id (user_id),
    INDEX idx_key_hash (key_hash)
);
```

**subscription_plans**
```sql
CREATE TABLE subscription_plans (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL, -- free, base, pro, enterprise
    price_cents INT NOT NULL,
    stripe_price_id VARCHAR(255),
    max_cloud_accounts INT, -- NULL for unlimited
    max_database_connections INT, -- NULL for unlimited
    rate_limit_per_minute INT NOT NULL,
    features JSON, -- Array of feature flags
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**stripe_customers**
```sql
CREATE TABLE stripe_customers (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) UNIQUE NOT NULL,
    stripe_customer_id VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    INDEX idx_stripe_customer_id (stripe_customer_id)
);
```

**stripe_subscriptions**
```sql
CREATE TABLE stripe_subscriptions (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    stripe_subscription_id VARCHAR(255) UNIQUE NOT NULL,
    plan_id VARCHAR(36) NOT NULL,
    status VARCHAR(50) NOT NULL, -- active, canceled, past_due, trialing
    current_period_start DATETIME NOT NULL,
    current_period_end DATETIME NOT NULL,
    cancel_at_period_end BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    FOREIGN KEY (plan_id) REFERENCES subscription_plans(id),
    INDEX idx_account_id (account_id),
    INDEX idx_status (status)
);
```

**stripe_invoices**
```sql
CREATE TABLE stripe_invoices (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    stripe_invoice_id VARCHAR(255) UNIQUE NOT NULL,
    amount_cents INT NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(50) NOT NULL, -- draft, open, paid, void, uncollectible
    invoice_pdf_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    INDEX idx_account_id (account_id),
    INDEX idx_created_at (created_at)
);
```

**stripe_payments**
```sql
CREATE TABLE stripe_payments (
    id VARCHAR(36) PRIMARY KEY,
    invoice_id VARCHAR(36) NOT NULL,
    stripe_payment_intent_id VARCHAR(255) UNIQUE NOT NULL,
    amount_cents INT NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(50) NOT NULL, -- succeeded, failed, pending
    payment_method VARCHAR(50), -- card, bank_transfer
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (invoice_id) REFERENCES stripe_invoices(id),
    INDEX idx_invoice_id (invoice_id),
    INDEX idx_status (status)
);
```


**cloud_accounts**
```sql
CREATE TABLE cloud_accounts (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    provider VARCHAR(20) NOT NULL, -- aws, azure, gcp
    account_name VARCHAR(255) NOT NULL,
    encrypted_credentials TEXT NOT NULL, -- AES-256 encrypted JSON
    status VARCHAR(50) DEFAULT 'active', -- active, inactive, error
    last_sync_at DATETIME,
    last_sync_status VARCHAR(50), -- success, failed
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    INDEX idx_account_id (account_id),
    INDEX idx_provider (provider),
    INDEX idx_last_sync_at (last_sync_at)
);
```

**cloud_costs**
```sql
CREATE TABLE cloud_costs (
    id VARCHAR(36) PRIMARY KEY,
    cloud_account_id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    resource_id VARCHAR(255),
    cost_amount DECIMAL(15, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    region VARCHAR(50),
    tags JSON, -- Resource tags for filtering
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cloud_account_id) REFERENCES cloud_accounts(id),
    INDEX idx_cloud_account_date (cloud_account_id, date),
    INDEX idx_date (date),
    INDEX idx_service_name (service_name)
);
```

**cost_anomalies**
```sql
CREATE TABLE cost_anomalies (
    id VARCHAR(36) PRIMARY KEY,
    cloud_account_id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    baseline_cost DECIMAL(15, 2) NOT NULL,
    actual_cost DECIMAL(15, 2) NOT NULL,
    deviation_percentage DECIMAL(5, 2) NOT NULL,
    severity VARCHAR(20) NOT NULL, -- low, medium, high
    contributing_services JSON, -- Array of service names
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_by VARCHAR(36),
    acknowledged_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cloud_account_id) REFERENCES cloud_accounts(id),
    INDEX idx_cloud_account_date (cloud_account_id, date),
    INDEX idx_severity (severity),
    INDEX idx_acknowledged (acknowledged)
);
```

**database_connections**
```sql
CREATE TABLE database_connections (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    db_type VARCHAR(50) NOT NULL, -- postgresql, mysql, mongodb, sqlserver
    connection_name VARCHAR(255) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL,
    database_name VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    encrypted_password TEXT NOT NULL, -- AES-256 encrypted
    ssl_enabled BOOLEAN DEFAULT TRUE,
    status VARCHAR(50) DEFAULT 'active', -- active, inactive, error
    last_schema_sync DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    INDEX idx_account_id (account_id),
    INDEX idx_db_type (db_type)
);
```

**query_logs**
```sql
CREATE TABLE query_logs (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    database_connection_id VARCHAR(36) NOT NULL,
    query_text TEXT NOT NULL,
    generated_sql TEXT,
    execution_time_ms INT,
    result_count INT,
    status VARCHAR(50) NOT NULL, -- success, error, timeout
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (database_connection_id) REFERENCES database_connections(id),
    INDEX idx_user_id (user_id),
    INDEX idx_created_at (created_at),
    INDEX idx_status (status)
);
```

**dashboards**
```sql
CREATE TABLE dashboards (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    layout_config JSON NOT NULL, -- Widget positions and sizes
    visible_widgets JSON NOT NULL, -- Array of enabled widget IDs
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user_id (user_id)
);
```

**mail_settings**
```sql
CREATE TABLE mail_settings (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) UNIQUE NOT NULL,
    smtp_host VARCHAR(255) NOT NULL,
    smtp_port INT NOT NULL,
    smtp_username VARCHAR(255) NOT NULL,
    encrypted_password TEXT NOT NULL, -- AES-256 encrypted
    from_email VARCHAR(255) NOT NULL,
    use_tls BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);
```

**audit_logs**
```sql
CREATE TABLE audit_logs (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36),
    account_id VARCHAR(36),
    action_type VARCHAR(100) NOT NULL, -- login, logout, create, update, delete
    resource_type VARCHAR(100) NOT NULL, -- user, subscription, cloud_account, etc.
    resource_id VARCHAR(36),
    old_value JSON,
    new_value JSON,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_account_id (account_id),
    INDEX idx_action_type (action_type),
    INDEX idx_created_at (created_at)
);
```

### Data Relationships

**Multi-Tenancy:**
- All tenant-scoped tables include `account_id` foreign key
- Queries automatically filter by authenticated user's `account_id`
- Super_Admin can bypass account filtering for support

**User-Account Relationship:**
- One Account has many Users
- One User belongs to one Account
- Users have Roles through `user_roles` junction table

**Subscription-Account Relationship:**
- One Account has one active Subscription
- Subscription references a Plan
- Plan defines limits (cloud accounts, databases, rate limits)

**Cloud Cost Hierarchy:**
- Account → Cloud_Accounts → Cloud_Costs
- Cloud_Accounts → Cost_Anomalies
- Costs aggregated by date, service, region

**Query Execution Flow:**
- User → Database_Connection → Query_Logs
- Query results cached in Redis (not persisted in MySQL)

