import { Alert, AlertTitle, Button, Stack } from "@mui/material";
import { ApiError } from "../api/client";

const CODE_MESSAGES: Record<string, string> = {
  UNAUTHORIZED: "Требуется вход в систему. Авторизация появится в следующем этапе.",
  FORBIDDEN: "Недостаточно прав для выполнения операции.",
  VERSION_CONFLICT:
    "Данные были изменены другим запросом. Обновите страницу и попробуйте снова.",
  RESOURCE_IN_USE: "Ресурс используется и не может быть удалён.",
  ALREADY_EXISTS: "Запись с такими данными уже существует.",
};

export function mapApiErrorMessage(error: ApiError): string {
  if (error.code in CODE_MESSAGES) {
    return CODE_MESSAGES[error.code];
  }
  return error.message;
}

export function ApiErrorAlert({
  error,
  onRetry,
}: {
  error: ApiError;
  onRetry?: () => void;
}) {
  const title =
    error.code === "UNAUTHORIZED" || error.code === "FORBIDDEN"
      ? "Доступ"
      : "Ошибка запроса";

  return (
    <Alert
      severity="error"
      action={
        onRetry ? (
          <Button color="inherit" size="small" onClick={onRetry}>
            Повторить
          </Button>
        ) : undefined
      }
    >
      <AlertTitle>{title}</AlertTitle>
      {mapApiErrorMessage(error)}
      {error.requestId && (
        <Stack component="span" sx={{ display: "block", mt: 1, opacity: 0.8 }}>
          ID запроса: {error.requestId}
        </Stack>
      )}
    </Alert>
  );
}
