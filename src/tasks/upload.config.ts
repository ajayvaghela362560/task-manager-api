import { BadRequestException } from '@nestjs/common';
import type { MulterOptions } from '@nestjs/platform-express/multer/interfaces/multer-options.interface';
import { diskStorage } from 'multer';
import { randomUUID } from 'node:crypto';
import { extname } from 'node:path';

// Read at import time because interceptor options are bound when the
// controller class is decorated, before Nest's ConfigModule initializes.
export const UPLOAD_DIR = process.env.UPLOAD_DIR ?? './uploads';
export const MAX_UPLOAD_BYTES = 5 * 1024 * 1024;

const ALLOWED_EXTENSIONS = new Set([
  '.png',
  '.jpg',
  '.jpeg',
  '.gif',
  '.webp',
  '.pdf',
  '.txt',
  '.md',
  '.doc',
  '.docx',
]);

export const uploadOptions: MulterOptions = {
  // Multer creates the directory when destination is a string.
  storage: diskStorage({
    destination: UPLOAD_DIR,
    filename: (_req, file, cb) =>
      cb(null, randomUUID() + extname(file.originalname).toLowerCase()),
  }),
  limits: { fileSize: MAX_UPLOAD_BYTES },
  fileFilter: (_req, file, cb) => {
    if (!ALLOWED_EXTENSIONS.has(extname(file.originalname).toLowerCase())) {
      cb(
        new BadRequestException({
          code: 'unsupported_type',
          message:
            'Allowed file types: png, jpg, jpeg, gif, webp, pdf, txt, md, doc, docx',
        }),
        false,
      );
      return;
    }
    cb(null, true);
  },
};
