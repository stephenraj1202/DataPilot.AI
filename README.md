# SaaS FinOps Analytics Platform

A production-grade, multi-tenant cloud cost management and AI-powered database analytics system.

## Architecture

The platform consists of 5 microservices:

- **API Gateway** (Port 8080) - Request routing, authentication, rate limiting
- **Auth Service** (Port 8081) - User authentication, JWT tokens, RBAC
- **Billing Service** (Port 8082) - Stripe integration, subscription management
- **FinOps Service** (Port 8083) - Cloud cost aggregation, anomaly detection
- **AI Query Engine** (Port 8084) - Natural language to SQL conversion

## Technology Stack

**Backend:**
- Go 1.21 with Gin framework (API Gateway, Auth, Billing, FinOps)
- Python 3.11 with FastAPI (AI Query Engine)

**Data Layer:**
- MySQL 8.0 (Primary database)
- Redis 7.0 (Caching and rate limiting)

**External Integrations:**
- Stripe API (Payment processing)
- AWS, Azure, GCP APIs (Cloud cost data)
- OpenAI API (Natural language processing)

## Project Structure

```
.
├── services/
│   ├── api-gateway/       # API Gateway service
│   ├── auth-service/      # Authentication service
│   ├── billing-service/   # Billing and subscriptions
│   ├── finops-service/    # Cloud cost management
│   └── ai-query-engine/   # AI-powered query engine (Python)
├── shared/
│   ├── config/            # Shared configuration loader
│   ├── database/          # MySQL connection pool
│   └── redis/             # Redis client with pooling
├── migrations/            # Database migration scripts
├── config.ini             # Configuration file
└── go.work                # Go workspace file
```

## Getting Started

### Prerequisites

- Go 1.21+
- Python 3.11+
- MySQL 8.0+
- Redis 7.0+

### Configuration

1. Copy `config.ini` and update with your settings:
   - Database credentials
   - Redis connection
   - Stripe API keys
   - SMTP settings
   - OpenAI API key
   - AES encryption key (32 bytes)

2. For AI Query Engine, copy `.env.example` to `.env` in `services/ai-query-engine/`

### Database Setup

Run migrations to create all required tables:

```bash
cd migrations
go run migrate.go
```

### Running Services

**Go Services:**

```bash
# API Gateway
cd services/api-gateway
go run main.go

# Auth Service
cd services/auth-service
go run main.go

# Billing Service
cd services/billing-service
go run main.go

# FinOps Service
cd services/finops-service
go run main.go
```

**Python AI Query Engine:**

```bash
cd services/ai-query-engine
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt
python main.py
```

### Health Checks

Each service exposes a `/health` endpoint:

- API Gateway: http://localhost:8080/health
- Auth Service: http://localhost:8081/health
- Billing Service: http://localhost:8082/health
- FinOps Service: http://localhost:8083/health
- AI Query Engine: http://localhost:8084/health

## Development

### Go Workspace

The project uses Go workspaces for managing multiple modules:

```bash
go work sync
```

### Installing Dependencies

**Go services:**
```bash
cd services/<service-name>
go mod download
```

**Python service:**
```bash
cd services/ai-query-engine
pip install -r requirements.txt
```

## Database Schema

The platform uses MySQL with the following main tables:

- `accounts` - Multi-tenant organizations
- `users` - User accounts with authentication
- `roles` & `permissions` - RBAC system
- `subscription_plans` - Billing tiers
- `stripe_*` - Stripe integration tables
- `cloud_accounts` & `cloud_costs` - FinOps data
- `database_connections` - External database connections
- `query_logs` - AI query execution logs
- `audit_logs` - System audit trail

## Security

- Passwords hashed with bcrypt (cost factor 12)
- JWT tokens for authentication (15-min access, 7-day refresh)
- API keys hashed before storage
- Database credentials encrypted with AES-256
- Rate limiting per subscription tier
- Multi-tenant data isolation

## License

Proprietary - All rights reserved
