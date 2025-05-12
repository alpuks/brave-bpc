import Cookies from "js-cookie";
import { createContext, useContext, useState, ReactNode } from "react";

interface User {
  character_name: string;
  auth_level: string;
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
  // FIX: This is a temporary fix for cookie not being in JSON format
  const cookie = Cookies.get('brave-bpc')

  const authCookie = cookie
    ? cookie.slice(1, -1).split(/\s?,\s?/)
      .map(item => item.split(':'))
      .reduce((a, [key, val]) => Object.assign(a, { [key]: val }), {}) as User : null

  const [user, setUser] = useState<User | null>(authCookie);

  const logout = () => {
    setUser(null);
    Cookies.remove('brave-bpc')
  };

  return (
    <AuthContext.Provider
      value={{ user, isAuthenticated: !!user, logout }}
    >
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
