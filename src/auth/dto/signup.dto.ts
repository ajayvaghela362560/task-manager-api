import {
  IsEmail,
  IsString,
  Matches,
  MaxLength,
  MinLength,
} from 'class-validator';

export class SignupDto {
  @MaxLength(100, { message: 'name must be at most 100 characters' })
  @Matches(/\S/, { message: 'name is required' })
  @IsString({ message: 'name is required' })
  name: string;

  @IsEmail({}, { message: 'a valid email address is required' })
  email: string;

  @MaxLength(72, { message: 'password must be at most 72 characters' })
  @MinLength(8, { message: 'password must be at least 8 characters' })
  @IsString({ message: 'password is required' })
  password: string;
}
