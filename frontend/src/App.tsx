import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AdminLayout } from "./app/AdminLayout";
import { AuthLoading } from "./features/auth/AuthLoading";
import { AuthProvider } from "./features/auth/AuthProvider";
import { RequireAuth } from "./features/auth/RequireAuth";
import { RequireAdmin } from "./features/auth/RequireRole";
import { AuthCallbackPage } from "./pages/AuthCallbackPage";
import { AuthErrorPage } from "./pages/AuthErrorPage";
import { LoginPage } from "./pages/LoginPage";
import { LogoutPage } from "./pages/LogoutPage";

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

export function AppRoutes() {
  return (
    <Suspense fallback={<AuthLoading />}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/auth/callback" element={<AuthCallbackPage />} />
        <Route path="/auth/error" element={<AuthErrorPage />} />

        <Route element={<RequireAuth />}>
          <Route path="/logout" element={<LogoutPage />} />
          <Route path="/" element={<Navigate to="/ingredients" replace />} />

          <Route element={<AdminLayout />}>
            <Route path="/ingredients" element={<IngredientsListPage />} />
            <Route path="/ingredients/:id" element={<IngredientDetailPage />} />

            <Route element={<RequireAdmin />}>
              <Route path="/ingredients/new" element={<IngredientFormPage />} />
              <Route
                path="/ingredients/:id/edit"
                element={<IngredientFormPage />}
              />
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
}

function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
