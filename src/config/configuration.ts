export interface AppConfig {
  port: number;
  databaseUrl: string;
  jwtSecret: string;
  jwtTtl: string;
  corsOrigins: string[];
  uploadDir: string;
  adminEmails: string[];
  maxUploadMb: number;
}

export default (): AppConfig => {
  const required = (key: string): string => {
    const value = process.env[key];
    if (!value) throw new Error(`${key} is required`);
    return value;
  };

  return {
    port: parseInt(process.env.PORT ?? '8080', 10),
    databaseUrl: required('DATABASE_URL'),
    jwtSecret: required('JWT_SECRET'),
    // Token lifetime in ms-package format, e.g. "168h" = 7 days.
    jwtTtl: process.env.JWT_TTL ?? '168h',
    corsOrigins: (process.env.CORS_ORIGIN ?? 'http://localhost:3000')
      .split(',')
      .map((o) => o.trim())
      .filter(Boolean),
    uploadDir: process.env.UPLOAD_DIR ?? './uploads',
    // Accounts that sign up with one of these emails get the admin role.
    adminEmails: (process.env.ADMIN_EMAILS ?? '')
      .split(',')
      .map((e) => e.trim().toLowerCase())
      .filter(Boolean),
    maxUploadMb: 5,
  };
};
