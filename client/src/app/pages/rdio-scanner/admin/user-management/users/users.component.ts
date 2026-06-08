import { Component, OnInit, inject } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AdminAccessCode, AdminRole, AdminUser, UserManagementService } from '../user-management.service';

@Component({
    selector: 'rdio-scanner-admin-users-page',
    styleUrls: ['./users.component.scss'],
    templateUrl: './users.component.html',
    standalone: false,
})
export class RdioScannerAdminUsersPageComponent implements OnInit {
    private readonly formBuilder = inject(FormBuilder);

    readonly displayedColumns: string[] = ['username', 'email', 'roles', 'status', 'actions'];
    readonly accessCodeColumns: string[] = ['label', 'code', 'createdAt', 'actions'];

    users: AdminUser[] = [];
    roles: AdminRole[] = [];
    accessCodes: AdminAccessCode[] = [];
    selectedUserId: number | null = null;

    form = this.formBuilder.group({
        email: this.formBuilder.nonNullable.control('', [Validators.required, Validators.email]),
        isSuspended: this.formBuilder.nonNullable.control(false),
        password: this.formBuilder.nonNullable.control(''),
        roles: this.formBuilder.nonNullable.control<number[]>([]),
        username: this.formBuilder.nonNullable.control('', [Validators.required]),
    });

    constructor(
        private userManagementService: UserManagementService,
        private matSnackBar: MatSnackBar,
    ) {}

    async ngOnInit(): Promise<void> {
        await this.reload();
    }

    edit(user: AdminUser): void {
        this.selectedUserId = user.id ?? null;
        this.form.setValue({
            email: user.email || '',
            isSuspended: !!user.isSuspended,
            password: '',
            roles: user.roles || [],
            username: user.username || '',
        });

        this.reloadSelectedUserCodes().catch(() => {
            this.accessCodes = [];
        });
    }

    resetForm(): void {
        this.selectedUserId = null;
        this.accessCodes = [];
        this.form.reset({
            email: '',
            isSuspended: false,
            password: '',
            roles: [],
            username: '',
        });
    }

    async save(): Promise<void> {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }

        const value = this.form.getRawValue();

        try {
            if (this.selectedUserId) {
                const updatePayload: {
                    email: string;
                    isSuspended: boolean;
                    password?: string;
                    roles: number[];
                    username: string;
                } = {
                    email: value.email,
                    isSuspended: value.isSuspended,
                    roles: value.roles,
                    username: value.username,
                };

                if (value.password.trim() !== '') {
                    updatePayload.password = value.password;
                }

                await this.userManagementService.updateUser(this.selectedUserId, {
                    ...updatePayload,
                });
                this.matSnackBar.open('User updated.', 'Close', { duration: 4000 });
            } else {
                if (!value.password) {
                    this.matSnackBar.open('Password is required for new users.', 'Close', { duration: 4000 });
                    return;
                }
                await this.userManagementService.createUser({
                    email: value.email,
                    isSuspended: value.isSuspended,
                    password: value.password,
                    roles: value.roles,
                    username: value.username,
                });
                this.matSnackBar.open('User created.', 'Close', { duration: 4000 });
            }

            this.resetForm();
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to save user.', 'Close', { duration: 5000 });
        }
    }

    async delete(user: AdminUser): Promise<void> {
        if (!user.id) {
            return;
        }

        try {
            await this.userManagementService.deleteUser(user.id);
            this.matSnackBar.open('User deleted.', 'Close', { duration: 4000 });
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to delete user.', 'Close', { duration: 5000 });
        }
    }

    async toggleSuspend(user: AdminUser): Promise<void> {
        if (!user.id) {
            return;
        }

        try {
            await this.userManagementService.updateUser(user.id, {
                isSuspended: !user.isSuspended,
            });
            this.matSnackBar.open(user.isSuspended ? 'User unsuspended.' : 'User suspended.', 'Close', { duration: 4000 });
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to update suspension status.', 'Close', { duration: 5000 });
        }
    }

    getRoleNames(user: AdminUser): string {
        const roleNames = (user.roles || [])
            .map((roleId) => this.roles.find((role) => role.id === roleId)?.name)
            .filter((name): name is string => !!name);

        return roleNames.length > 0 ? roleNames.join(', ') : 'None';
    }

    getStatusLabel(user: AdminUser): string {
        return user.isSuspended ? 'Suspended' : 'Active';
    }

    async generateAccessCode(): Promise<void> {
        if (!this.selectedUserId) {
            return;
        }

        const label = window.prompt('Label for new access code (example: Android Phone):', 'Admin Generated Code');
        if (label === null) {
            return;
        }

        try {
            await this.userManagementService.createUserAccessCode(this.selectedUserId, label);
            this.matSnackBar.open('Access code generated.', 'Close', { duration: 4000 });
            await this.reloadSelectedUserCodes();
        } catch {
            this.matSnackBar.open('Unable to generate access code.', 'Close', { duration: 5000 });
        }
    }

    async revokeAccessCode(code: AdminAccessCode): Promise<void> {
        if (!this.selectedUserId || !code?.id) {
            return;
        }

        try {
            await this.userManagementService.deleteUserAccessCode(this.selectedUserId, code.id);
            this.matSnackBar.open('Access code revoked.', 'Close', { duration: 4000 });
            await this.reloadSelectedUserCodes();
        } catch {
            this.matSnackBar.open('Unable to revoke access code.', 'Close', { duration: 5000 });
        }
    }

    formatAccessCodeCreatedAt(createdAt: string): string {
        if (!createdAt) {
            return '-';
        }

        const date = new Date(createdAt);
        if (Number.isNaN(date.getTime())) {
            return '-';
        }

        return date.toLocaleString();
    }

    private async reload(): Promise<void> {
        this.roles = await this.userManagementService.listRoles();
        this.users = await this.userManagementService.listUsers();
    }

    private async reloadSelectedUserCodes(): Promise<void> {
        if (!this.selectedUserId) {
            this.accessCodes = [];
            return;
        }

        this.accessCodes = await this.userManagementService.listUserAccessCodes(this.selectedUserId);
    }
}
