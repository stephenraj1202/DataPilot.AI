# Quick Start Guide

This guide will help you get the SaaS FinOps Analytics Platform running locally in under 10 minutes.

## Prerequisites

- Go 1.21 or higher
- Python 3.11 or higher
- Docker and Docker Compose (recommended)
- MySQL 8.0 (or use Docker)
- Redis 7.0 (or use Docker)

## Option 1: Docker Compose (Recommended)

The fastest way to get started:

```bash
# 1. Update configuration
cp config.ini.example config.ini
# Edit config.ini with your settings

# 2. Set up AI Query Engine environment
cp services/ai-query-engine/.env.example services/ai-query-engine/.env
# Edit .env with your OpenAI API key

# 3. Start all services
docker-compose up -d

# 4. Check service health
curl http://localhost:8080/health  # API Gateway
curl http://localhost:8081/health  # Auth Service
curl http://localhost:8082/health  # Billing Service
curl http://localhost:8083/health  # FinOps Service
curl http://localhost:8084/health  # AI Query Engine
```

## Option 2: Local Development

### Step 1: Install Dependencies

```bash
# Run the setup script
chmod +x scripts/setup.sh
./scripts/setup.sh
```

Or manually:

```bash
# Initialize Go workspace
go work sync

# Download Go dependencies
cd shared/config && go mod download && cd ../..
cd shared/database && go mod download && cd ../..
cd shared/redis && go mod download && cd ../..
cd services/api-gateway && go mod download && cd ../..
cd services/auth-service && go mod download && cd ../..
cd services/billing-service && go mod download && cd ../..
cd services/finops-service && go mod download && cd ../..

# Set up Python environment
cd services/ai-query-engine
python3 -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate
pip install -r requirements.txt
deactivate
cd ../..
```

### Step 2: Configure

```bash
# Update config.ini with your settings
# Key settings to update:
# - database credentials
# - stripe API keys
# - SMTP settings
# - OpenAI API key
# - AES encryption key (must be 32 bytes)

# Update AI Query Engine .env
cp services/ai-query-engine/.env.example services/ai-query-engine/.env
# Edit with your OpenAI API key
```

### Step 3: Start Database Services

```bash
# Option A: Using Docker
docker-compose up -d mysql redis

# Option B: Use your local MySQL and Redis
# Make sure they're running and accessible
```

### Step 4: Run Migrations

```bash
cd migrations
go run migrate.go
cd ..
```

### Step 5: Start Microservices

Open 5 terminal windows and run:

```bash
# Terminal 1: API Gateway
make run-api-gateway

# Terminal 2: Auth Service
make run-auth

# Terminal 3: Billing Service
make run-billing

# Terminal 4: FinOps Service
make run-finops

# Terminal 5: AI Query Engine
make run-ai
```

### Step 6: Verify

Check that all services are healthy:

```bash
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8082/health
curl http://localhost:8083/health
curl http://localhost:8084/health
```

## Project Structure

```
.
├── services/              # Microservices
│   ├── api-gateway/      # Port 8080
│   ├── auth-service/     # Port 8081
│   ├── billing-service/  # Port 8082
│   ├── finops-service/   # Port 8083
│   └── ai-query-engine/  # Port 8084
├── shared/               # Shared Go packages
│   ├── config/          # Configuration loader
│   ├── database/        # MySQL connection pool
│   └── redis/           # Redis client
├── migrations/          # Database migrations
├── config.ini          # Main configuration
└── docker-compose.yml  # Docker orchestration
```

## Next Steps

1. **Seed Initial Data**: Create roles, permissions, and subscription plans
2. **Test Authentication**: Register a user and obtain JWT tokens
3. **Connect Cloud Accounts**: Add AWS/Azure/GCP credentials
4. **Run AI Queries**: Connect external databases and test natural language queries
5. **Set up Stripe**: Configure webhook endpoints for payment processing

## Troubleshooting

### Database Connection Failed
- Check MySQL is running: `docker ps` or `mysql -u root -p`
- Verify credentials in `config.ini`
- Ensure database `finops_platform` exists

### Redis Connection Failed
- Check Redis is running: `docker ps` or `redis-cli ping`
- Verify host/port in `config.ini`

### Go Module Errors
```bash
go work sync
go clean -modcache
cd services/<service-name> && go mod download
```

### Python Dependencies Failed
```bash
cd services/ai-query-engine
python3 -m venv venv --clear
source venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt
```

## Development Tips

- Use `make help` to see all available commands
- Each service has a `/health` endpoint for monitoring
- Logs are written to stdout (use `docker-compose logs -f <service>`)
- Hot reload: Use `air` for Go services or `uvicorn --reload` for Python

## Support

For issues or questions, refer to:
- README.md for detailed documentation
- Design document: `.kiro/specs/saas-finops-analytics-platform/design.md`
- Requirements: `.kiro/specs/saas-finops-analytics-platform/requirements.md`
