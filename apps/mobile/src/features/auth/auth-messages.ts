import type { AuthAPIErrorField } from '@/lib/api/auth';

export type ReturnPath = '/' | '/compose' | '/profile';

export const genericLoginErrorMessage =
  'Não deu pra entrar agora. Tente de novo em instantes.';
export const genericSignupErrorMessage =
  'Não deu pra criar a conta agora. Tente de novo em instantes.';
export const usernameTakenErrorMessage =
  'Esse nome de usuário já está em uso.';

export function returnPathFromParam(
  value: string | string[] | undefined,
): ReturnPath {
  if (value === '/compose' || value === '/profile') {
    return value;
  }
  return '/';
}

export function loginValidationMessage(
  fields: readonly AuthAPIErrorField[],
): string | null {
  return validationMessage(fields, false);
}

export function signupValidationMessage(
  fields: readonly AuthAPIErrorField[],
): string | null {
  return validationMessage(fields, true);
}

function validationMessage(
  fields: readonly AuthAPIErrorField[],
  includesDisplayName: boolean,
): string | null {
  for (const field of fields) {
    const message = validationFieldMessage(field, includesDisplayName);
    if (message !== null) {
      return message;
    }
  }
  return null;
}

function validationFieldMessage(
  field: AuthAPIErrorField,
  includesDisplayName: boolean,
): string | null {
  switch (field.field) {
    case 'username':
      return usernameValidationMessage(field.code);
    case 'password':
      return passwordValidationMessage(field.code);
    case 'display_name':
      return includesDisplayName
        ? displayNameValidationMessage(field.code)
        : null;
    default:
      return null;
  }
}

function usernameValidationMessage(
  code: AuthAPIErrorField['code'],
): string | null {
  switch (code) {
    case 'taken':
      return usernameTakenErrorMessage;
    case 'too_short':
      return 'O nome de usuário precisa ter pelo menos 3 caracteres.';
    case 'too_long':
      return 'O nome de usuário precisa ter no máximo 32 caracteres.';
    case 'invalid':
      return 'Use letras, números, ponto, hífen ou sublinhado no nome de usuário.';
    case 'required':
      return 'Escolha um nome de usuário.';
    default:
      return null;
  }
}

function passwordValidationMessage(
  code: AuthAPIErrorField['code'],
): string | null {
  switch (code) {
    case 'too_short':
      return 'A senha precisa ter pelo menos 8 caracteres.';
    case 'too_long':
      return 'A senha precisa ter no máximo 128 caracteres.';
    case 'required':
      return 'Escreva uma senha.';
    default:
      return null;
  }
}

function displayNameValidationMessage(
  code: AuthAPIErrorField['code'],
): string | null {
  switch (code) {
    case 'too_long':
      return 'Seu nome precisa ter no máximo 60 caracteres.';
    case 'required':
      return 'Escreva seu nome.';
    default:
      return null;
  }
}
