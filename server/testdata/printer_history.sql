-- Synthetic data for printer history and backup/restore checks. Use only disposable databases.
insert into users (id,email,password_hash,display_name,status)
values ('remove-user','remove@example.com','hash','Remove','active'), ('other-user','other@example.com','hash','Other','active');
insert into printer_bindings (id,user_id,name,device_identifier,provider_user_id,status,created_at,updated_at)
values ('remove-device','remove-user','Desk printer','hardware-1',1,'connected',now(),now());
insert into print_jobs (id,user_id,printer_binding_id,title,source,content,status,provider_print_content_id,created_at,updated_at,next_status_check_at)
values ('completed-job','remove-user','remove-device','History','Manual','keep completed content','completed',1,now(),now(),null),
       ('queued-job','remove-user','remove-device','Queued','Manual','keep queued content','queued',2,now(),now(),now()-interval '1 minute'),
       ('pending-job','remove-user','remove-device','Pending','Manual','keep pending content','pending',null,now(),now(),null);
insert into plugin_installations (id,plugin_key,source_type,display_name,version,runtime_type,manifest_json,status,created_at,updated_at)
values ('remove-plugin','remove-test','upload','Source','1.0.0','python','{}','ready',now(),now());
insert into plugin_bindings (id,plugin_installation_id,user_id,enabled,status,created_at,updated_at)
values ('remove-binding','remove-plugin','remove-user',true,'connected',now(),now());
insert into plugin_items (id,user_id,plugin_installation_id,plugin_binding_id,external_id,title,source_label,blocks_json,status,fetched_at,created_at,updated_at)
values ('remove-item','remove-user','remove-plugin','remove-binding','item-1','Item','Source','[]','printed',now(),now(),now());
insert into print_schedules (id,user_id,plugin_installation_id,plugin_binding_id,title,frequency_type,timezone,hour,minute,device_id,enabled,next_run_at,lease_until,created_at,updated_at)
values ('remove-schedule','remove-user','remove-plugin','remove-binding','Morning','daily','UTC',0,0,'remove-device',true,now()-interval '1 minute',now()+interval '2 minutes',now(),now());
insert into print_schedule_deliveries (id,print_schedule_id,plugin_item_id,status,print_job_id,created_at,updated_at)
values ('remove-delivery','remove-schedule','remove-item','printed','completed-job',now(),now());
