import configparser, sys, subprocess, os

c = configparser.ConfigParser()
c.read('config.ini')
d = c['database'] if 'database' in c else {}
host = d.get('host', 'localhost')
port = d.get('port', '3306')
user = d.get('username', 'root')
pwd  = d.get('password', 'rootpassword')
name = d.get('database_name', 'finops_platform')

print(f"[INFO] Connecting to MySQL at {host}:{port} as {user} ...")

try:
    import pymysql
    conn = pymysql.connect(host=host, port=int(port), user=user, password=pwd)
    cur = conn.cursor()
    cur.execute(f"CREATE DATABASE IF NOT EXISTS `{name}` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
    conn.commit()
    conn.close()
    print(f"[OK] Database '{name}' is ready.")
except ImportError:
    print("[WARN] pymysql not installed globally. Skipping DB creation (will be handled by migration).")
except Exception as e:
    print(f"[WARN] Could not create database: {e}")
    print("[WARN] Ensure MySQL is running and credentials in config.ini are correct.")

# Export env vars for Go migration tool
env = os.environ.copy()
env['DB_HOST']     = host
env['DB_PORT']     = port
env['DB_USERNAME'] = user
env['DB_PASSWORD'] = pwd
env['DB_NAME']     = name

if os.path.exists('migrations/migrate.go'):
    print("[INFO] Running database migrations...")
    result = subprocess.run(
        ['go', 'run', 'migrate.go'],
        cwd='migrations',
        env=env
    )
    if result.returncode != 0:
        print("[ERROR] Migrations failed.")
        sys.exit(1)
    print("[OK] Migrations complete.")
else:
    print("[SKIP] migrations/migrate.go not found.")
