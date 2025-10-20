import {
  createContext,
  useContext,
  useState,
  ReactNode,
  useEffect,
  useCallback,
  useMemo,
} from "react";

interface User {
  character_name: string;
  auth_level: number;
  character_id: number;
}

type AuthStatus = "loading" | "authenticated" | "unauthenticated";

export interface AuthContextValue {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

interface AuthState {
  status: AuthStatus;
  user: User | null;
}

async function fetchSession(): Promise<User | null> {
  const response = await fetch("/session", {
    credentials: "include",
  });

  if (!response.ok) {
    return null;
  }

  const data = await response.json();
  return {
    character_name: data.character_name,
    auth_level: data.auth_level,
    character_id: Number(data.character_id),
  } satisfies User;
}

export const AuthProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [state, setState] = useState<AuthState>({
    status: "loading",
    user: null,
  });

  const refresh = useCallback(async () => {
    setState((prev) => ({ ...prev, status: "loading" }));
    try {
      const user = await fetchSession();
      setState({ status: user ? "authenticated" : "unauthenticated", user });
    } catch {
      setState({ status: "unauthenticated", user: null });
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const logout = useCallback(async () => {
    await fetch("/logout", { credentials: "include" });
    setState({ status: "unauthenticated", user: null });
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user: state.user,
      isAuthenticated: state.status === "authenticated",
      isLoading: state.status === "loading",
      refresh,
      logout,
    }),
    [state, refresh, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
