import {
  createContext,
  useContext,
  useState,
  ReactNode,
  useEffect,
} from "react";

interface User {
  character_name: string;
  auth_level: number;
  character_id: string;
}

export interface AuthContext {
  user: User | null;
  isAuthenticated: boolean;
  logout: () => void;
}

const AuthContext = createContext<AuthContext | null>(null);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [user, setUser] = useState<User | null>(null);
  useEffect(() => {
    const ac = new AbortController();
    (async () => {
      try {
        const res = await fetch("/session", {
          credentials: "include",
          signal: ac.signal,
        });
        if (!res.ok) {
          console.log("not authenticated");
          setUser(null);
        } else {
          const data = await res.json();
          setUser({
            character_name: data.character_name,
            auth_level: data.auth_level,
            character_id: data.character_id,
          });
        }
      } catch (err) {
        if ((err as Error).name !== "AbortError") {
          setUser(null);
        }
      }
    })();
    return () => ac.abort();
  }, []);
  const logout = async () => {
    await fetch("/logout", { credentials: "include" });
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, logout }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
