import { Avatar, Button, Stack, Typography } from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import { useAuth } from "./auth-context";

export function UserMenu() {
  const { user } = useAuth();
  if (!user) {
    return null;
  }
  return (
    <Stack
      direction="row"
      spacing={1}
      sx={{ alignItems: "center", px: 2, py: 1.5 }}
    >
      <Avatar
        src={user.avatar_url}
        alt={user.display_name}
        sx={{ width: 32, height: 32 }}
      />
      <Typography noWrap sx={{ flex: 1 }}>
        {user.display_name}
      </Typography>
      <Button component={RouterLink} to="/logout">
        Выйти
      </Button>
    </Stack>
  );
}
