"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { useRouter } from "next/navigation";
import { authApi, User } from "./api";
import { Zap } from "lucide-react";

interface AuthContextType {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    const storedToken = localStorage.getItem("token");
    if (storedToken) {
      setToken(storedToken);
      authApi
        .me()
        .then(setUser)
        .catch(() => {
          localStorage.removeItem("token");
          setToken(null);
        })
        .finally(() => setIsLoading(false));
    } else {
      setIsLoading(false);
    }
  }, []);

  const login = async (email: string, password: string) => {
    const response = await authApi.login(email, password);
    localStorage.setItem("token", response.token);
    setToken(response.token);
    setUser(response.user);
    router.push("/");
  };

  const register = async (email: string, password: string) => {
    const response = await authApi.register(email, password);
    localStorage.setItem("token", response.token);
    setToken(response.token);
    setUser(response.user);
    router.push("/");
  };

  const logout = () => {
    localStorage.removeItem("token");
    setToken(null);
    setUser(null);
    router.push("/login");
  };

  return (
    <AuthContext.Provider value={{ user, token, isLoading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, isLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && !user) {
      router.push("/login");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="flex h-screen flex-col items-center justify-center bg-background">
        {/* Animated background */}
        <div className="fixed inset-0 animated-gradient opacity-50" />
        <div className="fixed inset-0 bg-grid-gradient opacity-30" />

        {/* Floating orbs */}
        <div className="fixed left-1/4 top-1/4 h-[300px] w-[300px] rounded-full orb-purple animate-float blur-[100px]" />
        <div className="fixed bottom-1/4 right-1/4 h-[250px] w-[250px] rounded-full orb-cyan animate-float blur-[100px]" style={{ animationDelay: "-2s" }} />

        {/* Loading content */}
        <div className="relative z-10 flex flex-col items-center">
          <div className="relative mb-6">
            <div className="absolute inset-0 rounded-3xl gradient-primary opacity-50 blur-xl animate-pulse" />
            <div className="relative flex h-20 w-20 items-center justify-center rounded-3xl gradient-primary text-white shadow-2xl glow-multi">
              <Zap className="h-10 w-10 animate-pulse" />
            </div>
          </div>
          <h1 className="text-2xl font-bold">
            <span className="gradient-text">Reflow Gateway</span>
          </h1>
          <p className="mt-3 text-muted-foreground">Loading...</p>
        </div>
      </div>
    );
  }

  if (!user) {
    return null;
  }

  return <>{children}</>;
}
