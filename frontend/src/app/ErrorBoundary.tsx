import { Alert, Button, Stack, Typography } from "@mui/material";
import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = {
  children: ReactNode;
};

type State = {
  error?: Error;
};

export class ErrorBoundary extends Component<Props, State> {
  state: State = {};

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("UI error boundary", error, info);
  }

  private reset = () => {
    this.setState({ error: undefined });
  };

  render() {
    if (this.state.error) {
      return (
        <Stack spacing={2} sx={{ py: 4 }}>
          <Alert severity="error">
            <Typography variant="subtitle1">Неожиданная ошибка интерфейса</Typography>
            {this.state.error.message}
          </Alert>
          <Button onClick={this.reset}>Попробовать снова</Button>
        </Stack>
      );
    }
    return this.props.children;
  }
}
