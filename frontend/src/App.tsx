import LocalBarIcon from "@mui/icons-material/LocalBar";
import MenuIcon from "@mui/icons-material/Menu";
import ScienceIcon from "@mui/icons-material/Science";
import {
  AppBar,
  Box,
  CircularProgress,
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
} from "@mui/material";
import { lazy, Suspense, useState } from "react";
import {
  BrowserRouter,
  Link as RouterLink,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
const drawerWidth = 240;
const IngredientsListPage = lazy(() =>
  import("./pages/IngredientsListPage").then((module) => ({
    default: module.IngredientsListPage,
  })),
);
const IngredientDetailPage = lazy(() =>
  import("./pages/IngredientDetailPage").then((module) => ({
    default: module.IngredientDetailPage,
  })),
);
const IngredientFormPage = lazy(() =>
  import("./pages/IngredientFormPage").then((module) => ({
    default: module.IngredientFormPage,
  })),
);

function Navigation({ onNavigate }: { onNavigate: () => void }) {
  const location = useLocation();
  return (
    <Box>
      <Toolbar sx={{ gap: 1 }}>
        <LocalBarIcon color="primary" />
        <Typography variant="h6" sx={{ fontWeight: 700 }}>
          Mixologist
        </Typography>
      </Toolbar>
      <Divider />
      <List sx={{ px: 1 }}>
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
    </Box>
  );
}

function AdminLayout() {
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <Box sx={{ display: "flex", minHeight: "100vh" }}>
      <AppBar
        position="fixed"
        color="inherit"
        sx={{ display: { md: "none" }, boxShadow: 1 }}
      >
        <Toolbar>
          <IconButton
            edge="start"
            onClick={() => setMobileOpen(true)}
            aria-label="Открыть меню"
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" sx={{ ml: 1 }}>
            Mixologist
          </Typography>
        </Toolbar>
      </AppBar>
      <Drawer
        variant="temporary"
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        sx={{
          display: { xs: "block", md: "none" },
          "& .MuiDrawer-paper": { width: drawerWidth },
        }}
      >
        <Navigation onNavigate={() => setMobileOpen(false)} />
      </Drawer>
      <Drawer
        variant="permanent"
        sx={{
          display: { xs: "none", md: "block" },
          width: drawerWidth,
          flexShrink: 0,
          "& .MuiDrawer-paper": { width: drawerWidth, boxSizing: "border-box" },
        }}
      >
        <Navigation onNavigate={() => undefined} />
      </Drawer>
      <Box
        component="main"
        sx={{ flex: 1, minWidth: 0, pt: { xs: 10, md: 4 }, pb: 6 }}
      >
        <Container maxWidth="xl">
          <Suspense
            fallback={
              <Box sx={{ display: "flex", justifyContent: "center", py: 8 }}>
                <CircularProgress />
              </Box>
            }
          >
            <Routes>
              <Route
                path="/"
                element={<Navigate to="/ingredients" replace />}
              />
              <Route path="/ingredients" element={<IngredientsListPage />} />
              <Route path="/ingredients/new" element={<IngredientFormPage />} />
              <Route
                path="/ingredients/:id"
                element={<IngredientDetailPage />}
              />
              <Route
                path="/ingredients/:id/edit"
                element={<IngredientFormPage />}
              />
              <Route
                path="*"
                element={<Navigate to="/ingredients" replace />}
              />
            </Routes>
          </Suspense>
        </Container>
      </Box>
    </Box>
  );
}

function App() {
  return (
    <BrowserRouter>
      <AdminLayout />
    </BrowserRouter>
  );
}

export default App;
