-- Seed roles and permissions for RBAC
-- Requirements: 2.1, 2.7

-- ============================================================
-- Insert five roles
-- ============================================================
INSERT IGNORE INTO roles (id, name, description, created_at, updated_at) VALUES
    ('00000000-0000-0000-0000-000000000001', 'super_admin',    'Platform owner with global access to all accounts and billing',                  NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000002', 'account_owner',  'Primary user who owns an Account subscription; all modules except super_admin',   NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000003', 'admin',          'Read-write access to FinOps, AI Query Engine, and database connectors',           NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000004', 'user',           'Read-write access to AI Query Engine and read-only access to FinOps',             NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000005', 'viewer',         'Read-only access to all dashboards',                                              NOW(), NOW());

-- ============================================================
-- Insert permissions (resource:action)
-- Resources: finops, query, billing, settings
-- Actions:   read, write, delete, execute
-- ============================================================
INSERT IGNORE INTO permissions (id, name, resource, action, created_at) VALUES
    -- finops permissions
    ('10000000-0000-0000-0000-000000000001', 'finops:read',     'finops',    'read',    NOW()),
    ('10000000-0000-0000-0000-000000000002', 'finops:write',    'finops',    'write',   NOW()),
    ('10000000-0000-0000-0000-000000000003', 'finops:delete',   'finops',    'delete',  NOW()),
    ('10000000-0000-0000-0000-000000000004', 'finops:execute',  'finops',    'execute', NOW()),
    -- query permissions
    ('10000000-0000-0000-0000-000000000005', 'query:read',      'query',     'read',    NOW()),
    ('10000000-0000-0000-0000-000000000006', 'query:write',     'query',     'write',   NOW()),
    ('10000000-0000-0000-0000-000000000007', 'query:delete',    'query',     'delete',  NOW()),
    ('10000000-0000-0000-0000-000000000008', 'query:execute',   'query',     'execute', NOW()),
    -- billing permissions
    ('10000000-0000-0000-0000-000000000009', 'billing:read',    'billing',   'read',    NOW()),
    ('10000000-0000-0000-0000-000000000010', 'billing:write',   'billing',   'write',   NOW()),
    ('10000000-0000-0000-0000-000000000011', 'billing:delete',  'billing',   'delete',  NOW()),
    ('10000000-0000-0000-0000-000000000012', 'billing:manage',  'billing',   'manage',  NOW()),
    -- settings permissions
    ('10000000-0000-0000-0000-000000000013', 'settings:read',   'settings',  'read',    NOW()),
    ('10000000-0000-0000-0000-000000000014', 'settings:write',  'settings',  'write',   NOW()),
    ('10000000-0000-0000-0000-000000000015', 'settings:delete', 'settings',  'delete',  NOW()),
    ('10000000-0000-0000-0000-000000000016', 'settings:manage', 'settings',  'manage',  NOW());

-- ============================================================
-- Map permissions to roles
--
-- super_admin    : all permissions
-- account_owner  : all permissions except super_admin-specific
--                  (finops:*, query:*, billing:manage, settings:manage)
-- admin          : finops:read, finops:write,
--                  query:read, query:execute, query:write,
--                  settings:read, settings:write, billing:read
-- user           : finops:read,
--                  query:read, query:execute, query:write
-- viewer         : finops:read, query:read, billing:read, settings:read
-- ============================================================

-- super_admin gets every permission
INSERT IGNORE INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000001', id FROM permissions;

-- account_owner: all except billing:delete (super_admin-only destructive ops)
-- Effectively: finops:*, query:*, billing:read/write/manage, settings:*
INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001'), -- finops:read
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002'), -- finops:write
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000003'), -- finops:delete
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000004'), -- finops:execute
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000005'), -- query:read
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000006'), -- query:write
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000007'), -- query:delete
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000008'), -- query:execute
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000009'), -- billing:read
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000010'), -- billing:write
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000012'), -- billing:manage
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000013'), -- settings:read
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000014'), -- settings:write
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000015'), -- settings:delete
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000016'); -- settings:manage

-- admin: finops:read/write, query:read/write/execute, settings:read/write, billing:read
INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001'), -- finops:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000002'), -- finops:write
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000005'), -- query:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000006'), -- query:write
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000008'), -- query:execute
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000009'), -- billing:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000013'), -- settings:read
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000014'); -- settings:write

-- user: finops:read, query:read/write/execute
INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000001'), -- finops:read
    ('00000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000005'), -- query:read
    ('00000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000006'), -- query:write
    ('00000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000008'); -- query:execute

-- viewer: finops:read, query:read, billing:read, settings:read
INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES
    ('00000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000001'), -- finops:read
    ('00000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000005'), -- query:read
    ('00000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000009'), -- billing:read
    ('00000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000013'); -- settings:read
