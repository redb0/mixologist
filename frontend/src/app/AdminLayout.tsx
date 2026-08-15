import LocalBarIcon from "@mui/icons-material/LocalBar";
import MenuIcon from "@mui/icons-material/Menu";
import ScienceIcon from "@mui/icons-material/Science";
import {
  AppBar,
  Box,
  Container,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
  useMediaQuery,
} from "@mui/material";
import { Suspense, useState } from "react";
import { Link as RouterLink, Outlet, useLocation } from "react-router-dom";
import { AuthLoading } from "../features/auth/AuthLoading";
import { UserMenu } from "../features/auth/UserMenu";

const drawerWidth = 240;

function Navigation({
  onNavigate,
  showUserMenu,
}: {
  onNavigate: () => void;
  showUserMenu: boolean;
}) {
  const location = useLocation();
  return (
    <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Toolbar sx={{ gap: 1 }}>
        <LocalBarIcon color="primary" />
        <Typography variant="h6" sx={{ fontWeight: 700 }}>
          Mixologist
        </Typography>
      </Toolbar>
      <Divider />
      <List sx={{ px: 1, flex: 1 }}>
        <ListItemButton
          component={RouterLink}
          to="/ingredients"
          selected={location.pathname.startsWith("/ingredients")}
          onClick={onNavigate}
        >
          <ListItemIcon>
            <ScienceIcon />
          </ListItemIcon>
          <ListItemText primary="Ингредиенты" />
        </ListItemButton>
      </List>
      {showUserMenu ? (
        <>
          <Divider />
          <UserMenu />
        </>
      ) : null}
    </Box>
  );
}

export function AdminLayout() {
  const isDesktop = useMediaQuery("(min-width:900px)");
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <Box sx={{ display: "flex", minHeight: "100vh" }}>
      {isDesktop ? (
        <Drawer
          variant="permanent"
          sx={{
            width: drawerWidth,
            flexShrink: 0,
            "& .MuiDrawer-paper": {
              width: drawerWidth,
              boxSizing: "border-box",
            },
          }}
        >
          <Navigation onNavigate={() => undefined} showUserMenu />
        </Drawer>
      ) : (
        <>
          <AppBar position="fixed" color="inherit" sx={{ boxShadow: 1 }}>
            <Toolbar>
              <IconButton
                edge="start"
                onClick={() => setMobileOpen(true)}
                aria-label="Открыть меню"
              >
                <MenuIcon />
              </IconButton>
              <Typography variant="h6" sx={{ ml: 1, flex: 1 }}>
                Mixologist
              </Typography>
              <UserMenu />
            </Toolbar>
          </AppBar>
          <Drawer
            variant="temporary"
            open={mobileOpen}
            onClose={() => setMobileOpen(false)}
            sx={{ "& .MuiDrawer-paper": { width: drawerWidth } }}
          >
            <Navigation
              onNavigate={() => setMobileOpen(false)}
              showUserMenu={false}
            />
          </Drawer>
        </>
      )}
      <Box
        component="main"
        sx={{ flex: 1, minWidth: 0, pt: { xs: 10, md: 4 }, pb: 6 }}
      >
        <Container maxWidth="xl">
          <Suspense fallback={<AuthLoading />}>
            <Outlet />
          </Suspense>
        </Container>
      </Box>
    </Box>
  );
}
