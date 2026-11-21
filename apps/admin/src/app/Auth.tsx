import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { get, sessionKey } from "../api/client";
import type { Operator } from "../generated/models";

interface Session {
  token: string | null;
  operator: Operator | undefined;
  loading: boolean;
  error: Error | null;
  reload: () => void;
  canPage: (path: string) => boolean;
  canCall: (endpoint: string) => boolean;
  login: (token: string) => void;
  logout: () => void;
}

const AuthContext = createContext<Session | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState(() => sessionStorage.getItem(sessionKey));
  const queryClient = useQueryClient();
  const profile = useQuery({
    queryKey: ["operator-profile"],
    queryFn: () => get<Operator>("/auth/me"),
    enabled: token !== null,
    retry: false,
    staleTime: 0,
    refetchInterval: 30_000,
  });
  const operator = token ? profile.data : undefined;

  function login(nextToken: string) {
    sessionStorage.setItem(sessionKey, nextToken);
    setToken(nextToken);
  }

  const logout = useCallback(() => {
    sessionStorage.removeItem(sessionKey);
    queryClient.clear();
    setToken(null);
  }, [queryClient]);

  useEffect(() => {
    function refreshPermissions() {
      void queryClient.invalidateQueries({ queryKey: ["operator-profile"] });
    }

    window.addEventListener("auth-expired", logout);
    window.addEventListener("access-forbidden", refreshPermissions);

    return () => {
      window.removeEventListener("auth-expired", logout);
      window.removeEventListener("access-forbidden", refreshPermissions);
    };
  }, [logout, queryClient]);

  return (
    <AuthContext.Provider
      value={{
        token,
        operator,
        loading: profile.isPending,
        error: profile.error,
        reload: () => void profile.refetch(),
        canPage: (path) =>
          !!operator &&
          (operator.super_admin ||
            (path !== "/operators" && operator.pages.includes(path))),
        canCall: (endpoint) =>
          !!operator &&
          (operator.super_admin || operator.endpoints.includes(endpoint)),
        login,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useSession(): Session {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("AuthProvider 未初始化");
  }

  return context;
}
