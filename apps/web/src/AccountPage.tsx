import { t, useLocale } from "./i18n";
import { ProfileForm, SecurityForms } from "./AccountForms";
import OperationsPanel from "./OperationsPanel";
import AvatarEditor from "./AvatarEditor";
import { roleNames } from "./api";
import type { Organization } from "./api";

export type AccountSection = "profile" | "security" | "settings";
const copy = {
  profile: {
    get title() {
      return t("个人中心");
    },
    get description() {
      return t("查看你的账户信息与组织身份。");
    },
    get heading() {
      return t("账户资料");
    },
  },
  security: {
    get title() {
      return t("安全中心");
    },
    get description() {
      return t("查看账户的登录方式与安全功能。");
    },
    get heading() {
      return t("登录与安全");
    },
  },
  settings: {
    get title() {
      return t("设置");
    },
    get description() {
      return t("管理当前组织与账户偏好。");
    },
    get heading() {
      return t("组织设置");
    },
  },
};
export default function AccountPage({
  section,
  username,
  organization,
  onMembers,
}: {
  section: AccountSection;
  username: string;
  organization: Organization;
  onMembers: () => void;
}) {
  useLocale();
  const text = copy[section];
  return (
    <div className="account-page">
      <div className="page-heading">
        <div>
          <p className="eyebrow">YOUR ACCOUNT</p>
          <h1>{text.title}</h1>
          <p className="subtitle">{text.description}</p>
        </div>
      </div>
      <section className="panel">
        <div className="section-heading">
          <h2>{text.heading}</h2>
        </div>
        <div className="account-page-content">
          {section === "profile" && (
            <>
              <AvatarEditor username={username} />
              <ProfileForm />
              <dl>
                <div>
                  <dt>{t("用户名")}</dt>
                  <dd>{username}</dd>
                </div>
                <div>
                  <dt>{t("当前组织")}</dt>
                  <dd>{organization.name}</dd>
                </div>
                <div>
                  <dt>{t("组织角色")}</dt>
                  <dd>{roleNames[organization.role]}</dd>
                </div>
              </dl>
            </>
          )}
          {section === "security" && <SecurityForms />}
          {section === "settings" && (
            <>
              <dl>
                <div>
                  <dt>{t("当前组织")}</dt>
                  <dd>{organization.name}</dd>
                </div>
                <div>
                  <dt>{t("你的角色")}</dt>
                  <dd>{roleNames[organization.role]}</dd>
                </div>
              </dl>
              <button className="secondary" onClick={onMembers}>
                {t("管理组织与人员 →")}
              </button>
              <p className="form-hint">
                {t(
                  "切换或创建组织，请使用侧边栏顶部的组织菜单。其他个人偏好设置尚未开放。",
                )}
              </p>
            </>
          )}
        </div>
      </section>
      {section === "settings" &&
        ["owner", "admin"].includes(organization.role) && (
          <OperationsPanel
            key={organization.id}
            organizationID={organization.id}
          />
        )}
    </div>
  );
}
