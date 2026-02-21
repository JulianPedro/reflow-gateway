"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { DashboardLayout } from "@/components/dashboard-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Users,
  User,
  Shield,
  Edit2,
  Loader2,
  CheckCircle2,
  Crown,
  UserCog,
  Tag,
  RefreshCw,
} from "lucide-react";
import { useAuth } from "@/lib/auth";
import { usersApi, User as ApiUser } from "@/lib/api";

type UserData = ApiUser;

const roleColors: Record<string, { gradient: string; bg: string }> = {
  admin: { gradient: "from-amber-500 to-orange-500", bg: "bg-amber-500/10" },
  user: { gradient: "from-cyan-500 to-blue-500", bg: "bg-cyan-500/10" },
  viewer: { gradient: "from-emerald-500 to-green-500", bg: "bg-emerald-500/10" },
};

const cardColors = [
  { gradient: "from-violet-500 to-purple-500", glow: "hover:shadow-violet-500/20" },
  { gradient: "from-cyan-500 to-blue-500", glow: "hover:shadow-cyan-500/20" },
  { gradient: "from-emerald-500 to-green-500", glow: "hover:shadow-emerald-500/20" },
  { gradient: "from-amber-500 to-orange-500", glow: "hover:shadow-amber-500/20" },
  { gradient: "from-pink-500 to-rose-500", glow: "hover:shadow-pink-500/20" },
];

function UserCard({
  user,
  onEdit,
  onRecycle,
  isRecycling,
  index,
  currentUserId,
}: {
  user: UserData;
  onEdit: () => void;
  onRecycle: () => void;
  isRecycling: boolean;
  index: number;
  currentUserId?: string;
}) {
  const color = cardColors[index % cardColors.length];
  const roleColor = roleColors[user.role] || roleColors.user;
  const isCurrentUser = user.id === currentUserId;

  return (
    <Card
      className={`group relative overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl transition-all duration-500 hover:border-border/50 hover:bg-card/50 hover:shadow-xl ${color.glow}`}
      style={{ animationDelay: `${index * 0.1}s` }}
    >
      {/* Gradient accent */}
      <div className={`absolute left-0 top-0 h-full w-1.5 bg-gradient-to-b ${color.gradient}`} />

      <CardContent className="relative p-6 pl-8">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className={`relative flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br ${color.gradient} text-white shadow-lg`}>
              <span className="text-xl font-bold">{user.email.charAt(0).toUpperCase()}</span>
              {user.role === "admin" && (
                <span className="absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full bg-amber-500 text-white shadow-lg">
                  <Crown className="h-3 w-3" />
                </span>
              )}
            </div>
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <h3 className="text-lg font-semibold">{user.email}</h3>
                {isCurrentUser && (
                  <span className="rounded-full bg-violet-500/10 px-2 py-0.5 text-xs font-medium text-violet-500">
                    You
                  </span>
                )}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span className={`inline-flex items-center gap-1.5 rounded-lg ${roleColor.bg} px-2.5 py-1 text-xs font-medium`}>
                  <Shield className="h-3 w-3" />
                  {user.role}
                </span>
                {user.groups && user.groups.length > 0 && user.groups.map((group) => (
                  <span
                    key={group}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-muted/30 px-2.5 py-1 text-xs font-medium"
                  >
                    <Tag className="h-3 w-3" />
                    {group}
                  </span>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                Joined {new Date(user.created_at).toLocaleDateString()}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="gap-2 hover:bg-amber-500/10 hover:text-amber-500"
              onClick={onRecycle}
              disabled={isRecycling}
              title="Recycle MCP sessions"
            >
              <RefreshCw className={`h-4 w-4 ${isRecycling ? "animate-spin" : ""}`} />
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="gap-2 hover:bg-violet-500/10 hover:text-violet-500"
              onClick={onEdit}
            >
              <Edit2 className="h-4 w-4" />
              Edit
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export default function UsersPage() {
  const queryClient = useQueryClient();
  const { user: currentUser } = useAuth();
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [selectedUser, setSelectedUser] = useState<UserData | null>(null);
  const [editRole, setEditRole] = useState("");
  const [editGroups, setEditGroups] = useState("");
  const [saved, setSaved] = useState(false);

  const { data: users = [], isLoading } = useQuery<UserData[]>({
    queryKey: ["users"],
    queryFn: usersApi.list,
  });

  const recycleMutation = useMutation({
    mutationFn: (id: string) => usersApi.recycleSessions(id),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: { role?: string; groups?: string[] } }) =>
      usersApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      setSaved(true);
      setTimeout(() => {
        setIsEditOpen(false);
        setSelectedUser(null);
        setSaved(false);
      }, 1500);
    },
  });

  const handleEdit = (user: UserData) => {
    setSelectedUser(user);
    setEditRole(user.role);
    setEditGroups(user.groups?.join(", ") || "");
    setIsEditOpen(true);
  };

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedUser) {
      const groups = editGroups
        .split(",")
        .map((g) => g.trim())
        .filter((g) => g.length > 0);
      updateMutation.mutate({
        id: selectedUser.id,
        data: { role: editRole, groups },
      });
    }
  };

  // Check if current user is admin
  const isAdmin = currentUser?.role === "admin";

  if (!isAdmin) {
    return (
      <DashboardLayout
        title="Users"
        description="Manage user roles and groups"
      >
        <Card className="border-dashed border-border/30 bg-card/20">
          <CardContent className="flex flex-col items-center justify-center py-20 text-center">
            <div className="relative mb-6">
              <div className="absolute inset-0 animate-pulse rounded-full bg-gradient-to-br from-red-500/20 to-rose-500/20" />
              <div className="relative flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-red-500 to-rose-500 text-white shadow-lg">
                <Shield className="h-10 w-10" />
              </div>
            </div>
            <h3 className="mb-2 text-2xl font-bold">Access Denied</h3>
            <p className="max-w-md text-muted-foreground">
              You need admin privileges to manage users
            </p>
          </CardContent>
        </Card>
      </DashboardLayout>
    );
  }

  return (
    <DashboardLayout
      title="Users"
      description="Manage user roles and groups"
    >
      {
        isLoading ? (
          <div className="space-y-4" >
            {
              [1, 2, 3].map((i) => (
                <Card
                  key={i}
                  className="h-32 animate-pulse border-border/30 bg-card/20"
                />
              ))
            }
          </div >
        ) : users.length === 0 ? (
          <Card className="border-dashed border-border/30 bg-card/20">
            <CardContent className="flex flex-col items-center justify-center py-20 text-center">
              <div className="relative mb-6">
                <div className="absolute inset-0 animate-pulse rounded-full bg-gradient-to-br from-pink-500/20 to-rose-500/20" />
                <div className="relative flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-pink-500 to-rose-500 text-white shadow-lg">
                  <Users className="h-10 w-10" />
                </div>
              </div>
              <h3 className="mb-2 text-2xl font-bold">No users found</h3>
              <p className="max-w-md text-muted-foreground">
                Users will appear here once they register
              </p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-4">
            {users.map((user: UserData, index: number) => (
              <UserCard
                key={user.id}
                user={user}
                index={index}
                currentUserId={currentUser?.id}
                onEdit={() => handleEdit(user)}
                onRecycle={() => recycleMutation.mutate(user.id)}
                isRecycling={recycleMutation.isPending && recycleMutation.variables === user.id}
              />
            ))}
          </div>
        )
      }

      {/* Edit User Dialog */}
      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent className="border-border/30 bg-card/95 backdrop-blur-xl sm:max-w-md">
          <form onSubmit={handleSave}>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-violet-500 to-purple-500 text-white">
                  <UserCog className="h-4 w-4" />
                </div>
                Edit User
              </DialogTitle>
              <DialogDescription>
                Update role and groups for {selectedUser?.email}
              </DialogDescription>
            </DialogHeader>
            <div className="py-6">
              {saved ? (
                <div className="flex flex-col items-center justify-center py-8">
                  <div className="relative mb-4">
                    <div className="absolute inset-0 animate-ping rounded-full bg-emerald-500/30" />
                    <div className="relative flex h-16 w-16 items-center justify-center rounded-full bg-gradient-to-br from-emerald-500 to-green-500 text-white">
                      <CheckCircle2 className="h-8 w-8" />
                    </div>
                  </div>
                  <p className="text-lg font-semibold">User updated successfully!</p>
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="role" className="text-sm font-medium">
                      Role
                    </Label>
                    <select
                      id="role"
                      className="flex h-11 w-full rounded-lg border border-border/30 bg-background/50 px-3 py-2 text-sm ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      value={editRole}
                      onChange={(e) => setEditRole(e.target.value)}
                    >
                      <option value="user">User</option>
                      <option value="admin">Admin</option>
                      <option value="viewer">Viewer</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="groups" className="text-sm font-medium">
                      Groups
                    </Label>
                    <Input
                      id="groups"
                      placeholder="e.g., engineering, devops, platform"
                      value={editGroups}
                      onChange={(e) => setEditGroups(e.target.value)}
                      className="h-11 bg-background/50 border-border/30"
                    />
                    <p className="text-xs text-muted-foreground">
                      Separate multiple groups with commas
                    </p>
                  </div>
                </div>
              )}
            </div>
            {!saved && (
              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setIsEditOpen(false);
                    setSelectedUser(null);
                  }}
                  className="border-border/30"
                >
                  Cancel
                </Button>
                <Button type="submit" disabled={updateMutation.isPending} className="gap-2 gradient-primary border-0 text-white">
                  {updateMutation.isPending ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Saving...
                    </>
                  ) : (
                    "Save Changes"
                  )}
                </Button>
              </DialogFooter>
            )}
          </form>
        </DialogContent>
      </Dialog>
    </DashboardLayout >
  );
}
