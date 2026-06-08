import { Component, OnInit, inject } from '@angular/core';
import { FormBuilder, Validators } from '@angular/forms';
import { MatSnackBar } from '@angular/material/snack-bar';
import { AdminInvite, AdminRole, UserManagementService } from '../user-management.service';

@Component({
    selector: 'rdio-scanner-admin-invites-page',
    styleUrls: ['./invites.component.scss'],
    templateUrl: './invites.component.html',
    standalone: false,
})
export class RdioScannerAdminInvitesPageComponent implements OnInit {
    private readonly formBuilder = inject(FormBuilder);

    invites: AdminInvite[] = [];
    roles: AdminRole[] = [];
    selectedInviteId: number | null = null;

    form = this.formBuilder.group({
        expirationDate: this.formBuilder.control<Date | null>(null),
        inviteCode: this.formBuilder.nonNullable.control('', [Validators.required]),
        isUsed: this.formBuilder.nonNullable.control(false),
        roleId: this.formBuilder.nonNullable.control<number>(0, [Validators.min(1)]),
    });

    constructor(
        private userManagementService: UserManagementService,
        private matSnackBar: MatSnackBar,
    ) {}

    async ngOnInit(): Promise<void> {
        await this.reload();
    }

    edit(invite: AdminInvite): void {
        this.selectedInviteId = invite.id ?? null;
        this.form.setValue({
            expirationDate: invite.expirationDate ? new Date(invite.expirationDate * 1000) : null,
            inviteCode: invite.inviteCode || '',
            isUsed: !!invite.isUsed,
            roleId: invite.roleId || 0,
        });
    }

    resetForm(): void {
        this.selectedInviteId = null;
        this.form.reset({
            expirationDate: null,
            inviteCode: '',
            isUsed: false,
            roleId: 0,
        });
    }

    async save(): Promise<void> {
        if (this.form.invalid) {
            this.form.markAllAsTouched();
            return;
        }

        const value = this.form.getRawValue();
        const expirationDate = value.expirationDate ? Math.floor(value.expirationDate.getTime() / 1000) : undefined;

        try {
            if (this.selectedInviteId) {
                await this.userManagementService.updateInvite(this.selectedInviteId, {
                    expirationDate,
                    inviteCode: value.inviteCode,
                    isUsed: value.isUsed,
                    roleId: value.roleId,
                });
                this.matSnackBar.open('Invite updated.', 'Close', { duration: 4000 });
            } else {
                await this.userManagementService.createInvite({
                    expirationDate,
                    inviteCode: value.inviteCode,
                    roleId: value.roleId,
                });
                this.matSnackBar.open('Invite created.', 'Close', { duration: 4000 });
            }

            this.resetForm();
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to save invite.', 'Close', { duration: 5000 });
        }
    }

    async delete(invite: AdminInvite): Promise<void> {
        if (!invite.id) {
            return;
        }

        try {
            await this.userManagementService.deleteInvite(invite.id);
            this.matSnackBar.open('Invite deleted.', 'Close', { duration: 4000 });
            await this.reload();
        } catch {
            this.matSnackBar.open('Unable to delete invite.', 'Close', { duration: 5000 });
        }
    }

    generateInviteCode(): void {
        const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        const length = 8 + Math.floor(Math.random() * 5);
        const values = new Uint32Array(length);

        if (window?.crypto?.getRandomValues) {
            window.crypto.getRandomValues(values);
        } else {
            for (let i = 0; i < length; i++) {
                values[i] = Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
            }
        }

        let code = '';
        for (let i = 0; i < length; i++) {
            code += alphabet[values[i] % alphabet.length];
        }

        this.form.controls.inviteCode.setValue(code);
        this.form.controls.inviteCode.markAsDirty();
        this.form.controls.inviteCode.markAsTouched();
    }

    private async reload(): Promise<void> {
        this.roles = await this.userManagementService.listRoles();
        this.invites = await this.userManagementService.listInvites();
    }
}
