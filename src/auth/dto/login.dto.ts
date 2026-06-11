import { IsEmail, IsString } from 'class-validator';

export class LoginDto {
  @IsEmail({}, { message: 'a valid email address is required' })
  email: string;

  @IsString({ message: 'password is required' })
  password: string;
}
