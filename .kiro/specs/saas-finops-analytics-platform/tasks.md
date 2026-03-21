# Implementation Plan: SaaS FinOps Analytics Platform

## Overview

This implementation plan breaks down the SaaS FinOps Analytics Platform into discrete, incremental coding tasks. The platform consists of five microservices (API Gateway, Auth Service, Billing Service, FinOps Service, AI Query Engine) and a React frontend, with MySQL and Redis for data storage.

The implementation follows a bottom-up approach: core infrastructure → authentication → billing → FinOps → AI query → frontend integration → deployment.

## Tasks

- [x] 1. Set up project structure and shared infrastructure
  - Create monorepo directory structure with separate folders for each microservice
  - Set up Go modules for API Gateway, Auth Service, Billing Service, and FinOps Service
  - Set up Python virtual environment and dependencies for AI Query Engine
  - Create shared configuration package for reading config.ini files
  - Set up MySQL database connection pool with retry logic
  - Set up Redis client with connection pooling
  - Create database migration scripts for all tables defined in design
  - _Requirements: 13.1, 13.2, 14.3_

- [x] 1.1 Write unit tests for configuration loader
  - Test config.ini parsing for all sections (database, stripe, mail, ai)
  - Test error handling for missing configuration values
  - _Requirements: 13.1, 13.7_

- [x] 2. Implement Auth Service core functionality
  - [x] 2.1 Create user registration endpoint with email verification
    - Implement POST /auth/register endpoint
    - Generate verification token with 24-hour expiry
    - Hash passwords using bcrypt with cost factor 12
    - Store user record with email_verified=false
    - Send verification email via SMTP
    - _Requirements: 1.5, 24.1, 24.2, 24.3, 27.1, 27.2, 27.3_

  - [x] 2.2 Write unit tests for user registration
    - Test password validation rules (length, complexity, common passwords)
    - Test duplicate email handling
    - Test verification token generation
    - _Requirements: 27.1, 27.2, 27.3_

  - [x] 2.3 Create email verification endpoint
    - Implement GET /auth/verify-email endpoint
    - Validate verification token and expiry
    - Set email_verified=true on success
    - _Requirements: 24.4, 24.5_

  - [x] 2.4 Implement JWT token generation and validation
    - Create function to generate Access_Token (15-min expiry) and Refresh_Token (7-day expiry)
    - Include user_id, account_id, and roles in JWT claims
    - Implement token validation middleware
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 2.5 Write unit tests for JWT token handling
    - Test token generation with correct claims
    - Test token expiry validation
    - Test invalid token rejection
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 2.6 Create login endpoint
    - Implement POST /auth/login endpoint
    - Validate email and password
    - Check email_verified status
    - Return Access_Token and Refresh_Token
    - Log authentication attempt in audit_logs
    - _Requirements: 1.1, 24.6, 30.1_

  - [x] 2.7 Create token refresh endpoint
    - Implement POST /auth/refresh endpoint
    - Validate Refresh_Token
    - Issue new Access_Token
    - _Requirements: 1.2_

  - [x] 2.8 Implement password reset flow
    - Create POST /auth/forgot-password endpoint to generate reset token
    - Create POST /auth/reset-password endpoint to validate token and update password
    - Store reset_token with 1-hour expiry
    - Prevent password reuse (last 5 passwords)
    - _Requirements: 1.4, 27.5_

  - [x] 2.9 Implement API key management
    - Create POST /auth/api-keys endpoint to generate 32-character secure key
    - Store hashed API key in api_keys table
    - Create GET /auth/api-keys endpoint to list user's API keys
    - Create DELETE /auth/api-keys/{id} endpoint to revoke keys
    - Update last_used_at on API key usage
    - _Requirements: 1.7, 23.1, 23.2, 23.3, 23.4, 23.5_

  - [x] 2.10 Write unit tests for API key management
    - Test API key generation and hashing
    - Test API key validation
    - Test API key revocation
    - _Requirements: 23.1, 23.3, 23.5_

- [x] 3. Implement RBAC (Role-Based Access Control)
  - [x] 3.1 Create role and permission seeding script
    - Insert five roles: super_admin, account_owner, admin, user, viewer
    - Insert permissions for each resource (finops, query, billing, settings)
    - Map permissions to roles in role_permissions table
    - _Requirements: 2.1, 2.7_

  - [x] 3.2 Implement permission checking middleware
    - Create function to check if user has required permission
    - Query user_roles and role_permissions tables
    - Handle Super_Admin bypass for all permissions
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6_

  - [x] 3.3 Write unit tests for RBAC enforcement
    - Test permission checks for each role
    - Test Super_Admin bypass
    - Test permission denial for unauthorized roles
    - _Requirements: 2.2, 2.3, 2.4, 2.5, 2.6_

- [x] 4. Implement API Gateway
  - [x] 4.1 Create API Gateway routing infrastructure
    - Set up Gin framework with route groups
    - Implement request routing to Auth Service, Billing Service, FinOps Service, AI Query Engine
    - Add health check endpoint at /health
    - _Requirements: 17.1, 17.2, 17.3, 17.4, 18.5_

  - [x] 4.2 Implement authentication middleware
    - Extract Access_Token or API_Key from request headers
    - Validate token with Auth Service
    - Attach user_id and account_id to request context
    - Return HTTP 401 for invalid authentication
    - _Requirements: 9.1, 9.2_

  - [x] 4.3 Implement rate limiting middleware
    - Query user's subscription plan from Primary_Database
    - Enforce rate limits: Free (100/min), Base (500/min), Pro (2000/min), Enterprise (10000/min)
    - Use Redis to track request counts per user per minute
    - Return HTTP 429 with retry-after header when limit exceeded
    - _Requirements: 9.3, 9.4, 9.5, 9.6, 9.7_

  - [x] 4.4 Write unit tests for rate limiting
    - Test rate limit enforcement for each subscription tier
    - Test HTTP 429 response when limit exceeded
    - Test rate limit reset after time window
    - _Requirements: 9.3, 9.4, 9.5, 9.6, 9.7_

  - [x] 4.5 Implement circuit breaker pattern
    - Create circuit breaker with 5 failure threshold and 30-second timeout
    - Track service health for each microservice
    - Return HTTP 503 when circuit is open
    - _Requirements: 17.5, 17.6_

  - [x] 4.6 Add request logging
    - Log all requests with timestamp, user_id, endpoint, method, status_code, response_time
    - Store logs in audit_logs table
    - _Requirements: 17.7_

- [x] 5. Checkpoint - Ensure Auth Service and API Gateway tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Implement Billing Service with Stripe integration
  - [x] 6.1 Create subscription plan seeding script
    - Insert four plans: Free ($0), Base ($10), Pro ($20), Enterprise ($50)
    - Set plan limits: cloud accounts, database connections, rate limits
    - Store Stripe price IDs
    - _Requirements: 3.1, 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7_

  - [x] 6.2 Implement Stripe customer creation
    - Create POST /billing/customers endpoint
    - Call Stripe API to create customer
    - Store stripe_customer_id in stripe_customers table
    - Assign Free plan by default
    - _Requirements: 3.2_

  - [x] 6.3 Implement subscription creation endpoint
    - Create POST /billing/subscribe endpoint
    - Create Stripe subscription with payment method
    - Store subscription in stripe_subscriptions table
    - _Requirements: 3.1, 3.2_

  - [x] 6.4 Implement subscription upgrade/downgrade
    - Create PUT /billing/subscription endpoint
    - Calculate proration for upgrades
    - Apply changes immediately for upgrades, at period end for downgrades
    - Update stripe_subscriptions table
    - _Requirements: 3.3, 3.4_

  - [x] 6.5 Write unit tests for subscription management
    - Test subscription creation
    - Test upgrade proration calculation
    - Test downgrade scheduling
    - _Requirements: 3.3, 3.4_

  - [x] 6.6 Implement Stripe webhook handler
    - Create POST /billing/webhook endpoint
    - Verify Stripe signature using webhook_secret
    - Implement idempotency using event ID
    - Handle events: customer.subscription.created, customer.subscription.updated, customer.subscription.deleted, invoice.payment_succeeded, invoice.payment_failed
    - Respond within 5 seconds
    - _Requirements: 3.5, 26.1, 26.2, 26.3, 26.4, 26.5_

  - [x] 6.7 Implement payment failure notifications
    - Send email to Account_Owner and Super_Admin on payment failure
    - Store notification in audit_logs
    - _Requirements: 3.8_

  - [x] 6.8 Implement invoice management
    - Store invoices in stripe_invoices table on webhook events
    - Create GET /billing/invoices endpoint to list invoices
    - Create GET /billing/invoices/{id}/pdf endpoint to download invoice PDF
    - Send email with PDF attachment when invoice is generated
    - _Requirements: 3.6, 21.1, 21.2, 21.3, 21.4_

  - [x] 6.9 Write integration tests for webhook handling
    - Test webhook signature verification
    - Test event processing for each event type
    - Test idempotency
    - _Requirements: 26.1, 26.3, 26.4_

- [x] 7. Implement subscription plan enforcement
  - [x] 7.1 Create plan limit validation middleware
    - Query user's subscription plan
    - Check current usage against plan limits
    - Return error message prompting upgrade when limit exceeded
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8_

  - [x] 7.2 Write unit tests for plan enforcement
    - Test limit validation for each plan tier
    - Test error messages for limit exceeded
    - _Requirements: 12.8_

- [x] 8. Checkpoint - Ensure Billing Service tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Implement FinOps Service for cloud cost management
  - [x] 9.1 Create cloud account connection endpoint
    - Create POST /finops/cloud-accounts endpoint
    - Validate credentials for AWS, Azure, and GCP
    - Encrypt credentials using AES-256
    - Store connection in cloud_accounts table
    - _Requirements: 4.2, 8.3_

  - [x] 9.2 Write unit tests for credential encryption
    - Test AES-256 encryption and decryption
    - Test credential validation
    - _Requirements: 8.3_

  - [x] 9.3 Implement cloud cost sync scheduler
    - Create background job that runs every 6 hours
    - Iterate through all connected cloud accounts
    - Call AWS Cost Explorer, Azure Cost Management, and GCP Billing APIs
    - Extract daily cost data with service, resource, region breakdown
    - Store in cloud_costs table
    - Update last_sync_at and last_sync_status
    - Retry after 30 minutes on failure
    - _Requirements: 4.3, 4.4, 4.5_

  - [x] 9.4 Implement cost aggregation queries
    - Create GET /finops/costs/summary endpoint
    - Aggregate costs by provider, service, and time period
    - Support date range filtering
    - _Requirements: 4.6, 5.1_

  - [x] 9.5 Write unit tests for cost aggregation
    - Test cost summation by provider
    - Test cost summation by service
    - Test date range filtering
    - _Requirements: 4.6_

  - [x] 9.6 Implement cost anomaly detection
    - Calculate 30-day moving average baseline for each cloud account
    - Detect daily costs exceeding baseline by 20%
    - Create cost_anomalies record with severity level
    - Identify contributing services
    - _Requirements: 5.5, 19.1, 19.2, 19.5_

  - [x] 9.7 Implement anomaly notification system
    - Send email to Account_Owner and Admins when anomaly detected
    - Create GET /finops/anomalies endpoint to list anomalies
    - Support anomaly acknowledgment
    - _Requirements: 19.3, 19.4, 19.6_

  - [x] 9.8 Write unit tests for anomaly detection
    - Test baseline calculation
    - Test anomaly detection threshold (20%)
    - Test severity level assignment
    - _Requirements: 19.1, 19.2, 19.4_

  - [x] 9.9 Implement cost optimization recommendations
    - Identify idle resources (0% usage for 7 days)
    - Identify oversized resources (<20% utilization for 30 days)
    - Identify unattached storage volumes
    - Calculate potential monthly savings
    - Create GET /finops/recommendations endpoint
    - Update recommendations daily at 2:00 AM UTC
    - _Requirements: 22.1, 22.2, 22.3, 22.4, 22.5, 22.6_

  - [x] 9.10 Write unit tests for optimization recommendations
    - Test idle resource detection
    - Test oversized resource detection
    - Test savings calculation
    - _Requirements: 22.1, 22.2, 22.4_

- [x] 10. Checkpoint - Ensure FinOps Service tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Implement AI Query Engine with Python FastAPI
  - [x] 11.1 Set up FastAPI application structure
    - Create FastAPI app with CORS middleware
    - Set up database connection management
    - Add health check endpoint at /health
    - _Requirements: 18.5_

  - [x] 11.2 Implement database connection management
    - Create POST /query/connections endpoint to add database connections
    - Test connectivity before saving
    - Encrypt credentials using AES-256
    - Store in database_connections table
    - Support PostgreSQL, MySQL, MongoDB, SQL Server
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.6_

  - [x] 11.3 Write unit tests for database connection testing
    - Test connection validation for each database type
    - Test error message generation for connection failures
    - _Requirements: 8.4_

  - [x] 11.4 Implement schema extraction
    - Create GET /query/schema/{connection_id} endpoint
    - Extract table and column metadata from external databases
    - Support PostgreSQL, MySQL, MongoDB, SQL Server schema introspection
    - Complete extraction within 10 seconds
    - _Requirements: 8.5, 8.6_

  - [x] 11.5 Implement natural language to SQL conversion
    - Integrate OpenAI API or LangChain for LLM-based SQL generation
    - Construct prompt with database schema context
    - Generate SQL from natural language query within 3 seconds
    - Support PostgreSQL, MySQL, MongoDB, and T-SQL syntax
    - _Requirements: 6.1, 6.2, 6.3_

  - [ ] 11.6 Write unit tests for SQL generation
    - Test SQL generation for common query patterns
    - Test error handling for ambiguous queries
    - _Requirements: 6.1, 6.7_

  - [x] 11.7 Implement query execution with timeout
    - Execute generated SQL against external database
    - Enforce 30-second timeout
    - Cancel query and return timeout error if exceeded
    - Log query in query_logs table
    - _Requirements: 6.4, 6.6, 28.1, 28.2, 28.3_

  - [x] 11.8 Implement query result caching
    - Use Redis to cache query results for 5 minutes
    - Generate cache key from query text and connection ID
    - Return cached results within 100 milliseconds
    - Invalidate cache on schema changes
    - _Requirements: 20.1, 20.2, 20.3, 20.4, 20.5_

  - [ ] 11.9 Write unit tests for query caching
    - Test cache hit and miss scenarios
    - Test cache expiration
    - Test cache key generation
    - _Requirements: 20.1, 20.2, 20.3_

  - [x] 11.10 Implement chart type selection logic
    - Analyze query result structure
    - Determine appropriate chart type: metric, line, bar, pie, table
    - Format response with chartType, labels, data, rawData
    - _Requirements: 6.5, 7.1, 7.2, 7.3, 7.4_

  - [x] 11.11 Create query execution endpoint
    - Create POST /query/execute endpoint
    - Orchestrate: schema fetch → SQL generation → cache check → execution → chart type selection
    - Return structured JSON response
    - _Requirements: 6.1, 6.4, 6.5, 6.6_

  - [ ] 11.12 Write integration tests for query execution flow
    - Test end-to-end query execution
    - Test timeout handling
    - Test error handling for invalid SQL
    - _Requirements: 6.1, 6.4, 6.7, 28.1, 28.2_

- [x] 12. Checkpoint - Ensure AI Query Engine tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 13. Implement multi-tenant data isolation
  - [x] 13.1 Create account filtering middleware
    - Extract account_id from authenticated user context
    - Automatically filter all database queries by account_id
    - Implement Super_Admin bypass for support access
    - Log Super_Admin cross-account access in audit_logs
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5_

  - [ ] 13.2 Write unit tests for data isolation
    - Test account_id filtering
    - Test Super_Admin bypass
    - Test cross-account access prevention
    - _Requirements: 11.2, 11.3, 11.4_

- [x] 14. Implement custom SMTP configuration
  - [x] 14.1 Create SMTP settings management
    - Create POST /settings/smtp endpoint to save SMTP configuration
    - Encrypt SMTP password using AES-256
    - Store in mail_settings table
    - _Requirements: 10.3_

  - [x] 14.2 Implement email sending service
    - Check for custom SMTP configuration per account
    - Fall back to default platform SMTP if not configured
    - CC Super_Admin on all transactional emails
    - Implement retry logic with exponential backoff (3 retries)
    - Log email delivery failures
    - _Requirements: 10.1, 10.2, 10.4, 10.5, 10.6_

  - [ ] 14.3 Write unit tests for email service
    - Test SMTP configuration selection
    - Test retry logic
    - Test error logging
    - _Requirements: 10.1, 10.2, 10.6_

- [x] 15. Implement audit logging
  - [x] 15.1 Create audit logging service
    - Log authentication attempts
    - Log subscription changes with before/after values
    - Log database connection changes
    - Log role and permission changes
    - Store in audit_logs table with user_id, account_id, action_type, resource_type, ip_address, user_agent
    - _Requirements: 30.1, 30.2, 30.3, 30.4, 30.5_

  - [x] 15.2 Create audit log query endpoint
    - Create GET /admin/audit-logs endpoint for Super_Admin
    - Support filtering by account, user, action type, date range
    - _Requirements: 30.7_

- [x] 16. Implement Frontend with React and TypeScript
  - [x] 16.1 Set up React project structure
    - Create React app with TypeScript template
    - Set up folder structure: components, hooks, services, utils
    - Configure Material UI or ShadCN with Tailwind CSS
    - Set up React Router for navigation
    - _Requirements: 15.3_

  - [x] 16.2 Implement authentication pages
    - Create LoginForm component with email/password inputs
    - Create RegisterForm component with email verification flow
    - Create PasswordReset component for forgot password flow
    - Create EmailVerification page for verification link handling
    - Implement useAuth hook for authentication state management
    - _Requirements: 15.3, 24.3, 24.4_

  - [x] 16.3 Implement API service layer
    - Create axios instance with base URL and interceptors
    - Implement auth.service.ts for login, register, refresh token
    - Implement finops.service.ts for cost data and anomalies
    - Implement query.service.ts for AI query execution
    - Implement billing.service.ts for subscription management
    - Store Access_Token in memory and Refresh_Token in httpOnly cookie
    - _Requirements: 1.1, 1.2_

  - [x] 16.4 Implement theme management
    - Create theme context for dark/light mode
    - Store theme preference in localStorage
    - Apply theme to all components
    - _Requirements: 15.1, 15.2_

  - [x] 16.5 Create Dashboard page
    - Create DashboardGrid component with drag-and-drop layout
    - Create MetricCard component for quick stats
    - Implement dashboard customization (show/hide widgets)
    - Save dashboard configuration to backend
    - Implement lazy loading for dashboard components
    - _Requirements: 15.3, 15.7, 25.1, 25.2, 25.3, 25.4, 25.5_

  - [x] 16.6 Create FinOps Analytics page
    - Create CostChart component for line, bar, and pie charts
    - Display total monthly cost across all cloud accounts
    - Display cost distribution pie chart by service
    - Display daily cost bar chart
    - Display monthly trend line chart (12 months)
    - Create AnomalyAlert component for cost anomalies
    - Create RecommendationCard component for optimization suggestions
    - Refresh data every 5 minutes using React Query
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 29.2, 29.3_

  - [x] 16.7 Create AI Query Tool page
    - Create QueryInput component for natural language input
    - Create database connection selector dropdown
    - Create ResultChart component with auto-chart rendering
    - Support chart types: bar, line, pie, table, metric
    - Animate chart transitions over 300ms
    - Create QueryHistory component to display past queries
    - Implement CSV/JSON export functionality
    - _Requirements: 6.1, 6.5, 7.1, 7.2, 7.3, 7.4, 7.5, 7.6_

  - [x] 16.8 Create Billing page
    - Display current subscription plan
    - Display usage metrics vs plan limits
    - Create plan upgrade/downgrade UI
    - Display invoice history with download links
    - Implement payment method management
    - _Requirements: 15.3, 21.3_

  - [x] 16.9 Create Settings page
    - Create profile management section
    - Create team members and roles management
    - Create API key management UI
    - Create SMTP configuration form
    - Create database connection management
    - Create notification preferences
    - _Requirements: 15.3, 23.1, 23.5_

  - [x] 16.10 Create Landing page
    - Create marketing content sections
    - Create feature highlights
    - Create pricing table with plan comparison
    - Create sign-up CTA buttons
    - _Requirements: 15.3_

  - [x] 16.11 Implement loading and error states
    - Create LoadingSpinner component
    - Display loading indicators during async operations
    - Show toast notifications for success and error messages
    - _Requirements: 15.4, 15.5_

  - [x] 16.12 Implement responsive design
    - Ensure all pages work on desktop, tablet, and mobile
    - Test responsive breakpoints
    - _Requirements: 15.2_

  - [ ] 16.13 Write component tests for critical UI flows
    - Test authentication flow
    - Test dashboard customization
    - Test query execution and chart rendering
    - _Requirements: 15.3_

- [x] 17. Checkpoint - Ensure Frontend builds and runs
  - Ensure all tests pass, ask the user if questions arise.

- [x] 18. Implement containerization and deployment
  - [x] 18.1 Create Dockerfiles for each service
    - Create Dockerfile for API Gateway (Golang)
    - Create Dockerfile for Auth Service (Golang)
    - Create Dockerfile for Billing Service (Golang)
    - Create Dockerfile for FinOps Service (Golang)
    - Create Dockerfile for AI Query Engine (Python)
    - Create Dockerfile for Frontend (React)
    - Use multi-stage builds for optimized image sizes
    - _Requirements: 18.1, 18.3_

  - [x] 18.2 Create docker-compose.yml for local development
    - Define services: api-gateway, auth-service, billing-service, finops-service, ai-query-engine, frontend, mysql, redis
    - Configure environment variables for each service
    - Set up service dependencies and networking
    - Mount volumes for MySQL and Redis persistence
    - _Requirements: 18.2, 18.3_

  - [x] 18.3 Create Kubernetes deployment manifests
    - Create Deployment manifests for each microservice
    - Create Service manifests (LoadBalancer for API Gateway, ClusterIP for others)
    - Create StatefulSet for MySQL with persistent volume
    - Create Deployment for Redis with persistent volume
    - Configure resource limits and requests
    - Configure horizontal pod autoscaling
    - _Requirements: 18.4_

  - [x] 18.4 Create CI/CD pipeline configuration
    - Create GitHub Actions or GitLab CI workflow
    - Build and push Docker images on commit
    - Run tests before deployment
    - Deploy to staging and production environments
    - _Requirements: 18.4_

- [x] 19. Implement API documentation
  - [x] 19.1 Generate OpenAPI 3.0 specification
    - Document all API endpoints with request/response schemas
    - Include authentication examples (JWT and API Key)
    - Include rate limiting information per tier
    - Include webhook payload examples
    - _Requirements: 16.1, 16.2, 16.3, 16.4, 16.5_

  - [x] 19.2 Set up interactive API documentation
    - Serve OpenAPI spec at /docs endpoint
    - Configure Swagger UI or ReDoc for interactive exploration
    - Include SDK examples in Python, JavaScript, and Go
    - _Requirements: 16.6, 16.7_

- [x] 20. Final integration and testing
  - [x] 20.1 Test end-to-end user flows
    - Test user registration → email verification → login → dashboard
    - Test subscription upgrade → payment → plan enforcement
    - Test cloud account connection → cost sync → anomaly detection
    - Test database connection → AI query → chart rendering
    - _Requirements: All requirements_

  - [ ] 20.2 Write integration tests for critical paths
    - Test authentication flow
    - Test billing webhook processing
    - Test FinOps cost sync and anomaly detection
    - Test AI query execution with caching
    - _Requirements: All requirements_

  - [x] 20.3 Perform security audit
    - Verify JWT token validation
    - Verify API key hashing
    - Verify credential encryption (AES-256)
    - Verify rate limiting enforcement
    - Verify multi-tenant data isolation
    - Verify webhook signature verification
    - _Requirements: 1.6, 8.3, 9.1, 9.7, 11.2, 26.1_

  - [x] 20.4 Perform load testing
    - Test API Gateway under high request volume
    - Test rate limiting under concurrent requests
    - Test database connection pooling
    - Test Redis caching performance
    - _Requirements: 9.3, 9.4, 9.5, 9.6_

- [x] 21. Final checkpoint - Production readiness
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP delivery
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation and allow for user feedback
- The implementation follows a bottom-up approach: infrastructure → services → frontend → deployment
- All microservices should be developed in parallel after core infrastructure is complete
- Property-based tests are not included as the design focuses on integration and unit testing
- Security is prioritized throughout: encryption, authentication, authorization, rate limiting
- Multi-tenancy is enforced at the database query level with account_id filtering
