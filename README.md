# TaskFlow — Backend

NestJS 11 (TypeScript, TypeORM, PostgreSQL) REST API for TaskFlow.

See the [root README](../README.md) for full setup instructions.

```bash
npm install

# requires DATABASE_URL and JWT_SECRET (see .env.example)
npm run start:dev   # http://localhost:8080, migrations run automatically

npm test            # jest suite
npx eslint "src/**/*.ts"
npm run build
```
