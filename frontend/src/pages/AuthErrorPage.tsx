import { Button, Container, Stack } from "@mui/material";
import { Link as RouterLink, useSearchParams } from "react-router-dom";
import { ApiError } from "../shared/api/client";
import { ApiErrorAlert } from "../shared/ui/ApiErrorAlert";

export function AuthErrorPage() {
  const [params] = useSearchParams();
  const code = params.get("code") ?? "OAUTH_CALLBACK_FAILED";
  const message =
    params.get("message") ?? "Не удалось завершить вход через Google";
  const error = new ApiError(400, code, message);

  return (
    <Container maxWidth="sm" sx={{ py: 8 }}>
      <Stack spacing={2}>
        <ApiErrorAlert error={error} mapCode={false} />
        <Button
          component={RouterLink}
          to="/login"
          sx={{ alignSelf: "flex-start" }}
        >
          Вернуться ко входу
        </Button>
      </Stack>
    </Container>
  );
}
