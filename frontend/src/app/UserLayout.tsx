import LocalBarIcon from "@mui/icons-material/LocalBar";
import { AppBar, Box, Container, Toolbar, Typography } from "@mui/material";
import { Outlet } from "react-router-dom";
import { UserMenu } from "../features/auth/UserMenu";

export function UserLayout() {
  return (
    <Box sx={{ minHeight: "100vh" }}>
      <AppBar position="fixed" color="inherit" sx={{ boxShadow: 1 }}>
        <Toolbar sx={{ gap: 1 }}>
          <LocalBarIcon color="primary" />
          <Typography variant="h6" sx={{ fontWeight: 700, flex: 1 }}>
            Mixologist
          </Typography>
          <UserMenu />
        </Toolbar>
      </AppBar>
      <Box component="main" sx={{ pt: 10, pb: 6 }}>
        <Container maxWidth="md">
          <Outlet />
        </Container>
      </Box>
    </Box>
  );
}
