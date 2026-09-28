-- A normal member may add inventory, but cannot retarget or delete an enrolled machine.
DROP POLICY host_write ON hosts;
CREATE POLICY host_insert ON hosts FOR INSERT WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin','member'));
CREATE POLICY host_update ON hosts FOR UPDATE USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin')) WITH CHECK(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
CREATE POLICY host_delete ON hosts FOR DELETE USING(organization_id=request_org_id() AND tenant_role(organization_id) IN ('owner','admin'));
